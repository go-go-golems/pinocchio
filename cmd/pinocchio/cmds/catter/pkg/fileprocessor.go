package pkg

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/go-go-golems/pinocchio/pkg/filefilter"
	"github.com/weaviate/tiktoken-go"
)

type FileProcessor struct {
	MaxTotalSize   int64
	TotalSize      int64
	TotalTokens    int
	FileCount      int
	TokenCounter   *tiktoken.Tiktoken
	TokenCounts    map[string]int
	ListOnly       bool
	DelimiterType  string
	MaxLines       int
	MaxTokens      int
	Filter         *filefilter.FileFilter
	PrintFilters   bool
	Processor      middlewares.Processor
	Stats          *Stats
	OutputFormat   string
	OutputFile     string
	ArchivePrefix  string
	archiveWriter  io.Closer
	fileWriter     io.WriteCloser
	zipWriter      *zip.Writer
	tarWriter      *tar.Writer
	gzipWriter     *gzip.Writer
	archiveCounter *countingWriter
}

type FileProcessorOption func(*FileProcessor)

var (
	ErrMaxTokensExceeded    = errors.New("maximum total tokens limit reached")
	ErrMaxTotalSizeExceeded = errors.New("maximum total size limit reached")
)

func NewFileProcessor(options ...FileProcessorOption) *FileProcessor {
	tokenCounter, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error initializing tiktoken: %v\n", err)
		os.Exit(1)
	}

	fp := &FileProcessor{
		TokenCounter: tokenCounter,
		TokenCounts:  make(map[string]int),
		MaxLines:     0,
		MaxTokens:    0,
		PrintFilters: false,
	}

	for _, option := range options {
		option(fp)
	}

	return fp
}

func WithMaxTotalSize(size int64) FileProcessorOption {
	return func(fp *FileProcessor) {
		fp.MaxTotalSize = size
	}
}

func WithListOnly(listOnly bool) FileProcessorOption {
	return func(fp *FileProcessor) {
		fp.ListOnly = listOnly
	}
}

func WithDelimiterType(delimiterType string) FileProcessorOption {
	return func(fp *FileProcessor) {
		fp.DelimiterType = delimiterType
	}
}

func WithMaxLines(maxLines int) FileProcessorOption {
	return func(fp *FileProcessor) {
		fp.MaxLines = maxLines
	}
}

func WithMaxTokens(maxTokens int) FileProcessorOption {
	return func(fp *FileProcessor) {
		fp.MaxTokens = maxTokens
	}
}

func WithFileFilter(filter *filefilter.FileFilter) FileProcessorOption {
	return func(fp *FileProcessor) {
		fp.Filter = filter
	}
}

func WithPrintFilters(printFilters bool) FileProcessorOption {
	return func(fp *FileProcessor) {
		fp.PrintFilters = printFilters
	}
}

func WithProcessor(processor middlewares.Processor) FileProcessorOption {
	return func(fp *FileProcessor) {
		fp.Processor = processor
	}
}

func WithOutputFormat(format string) FileProcessorOption {
	return func(fp *FileProcessor) {
		fp.OutputFormat = format
	}
}

func WithOutputFile(file string) FileProcessorOption {
	return func(fp *FileProcessor) {
		fp.OutputFile = file
	}
}

func WithArchivePrefix(prefix string) FileProcessorOption {
	return func(fp *FileProcessor) {
		fp.ArchivePrefix = prefix
	}
}

func (fp *FileProcessor) ProcessPaths(paths []string) error {
	if fp.PrintFilters {
		fp.printConfiguredFilters()
		return nil
	}

	var err error

	if fp.Processor != nil {
		fp.Stats = NewStats()
		err = fp.Stats.ComputeStats(paths, fp.Filter)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error computing stats: %v\n", err)
			return err
		}
	}

	isArchiveOutput := fp.OutputFormat == "zip" || fp.OutputFormat == "tar.gz"
	if isArchiveOutput {
		if fp.OutputFile == "" {
			return fmt.Errorf("output file path is required for archive format")
		}
		outFile, err := os.Create(fp.OutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file %s: %w", fp.OutputFile, err)
		}
		fp.fileWriter = outFile
		fp.archiveCounter = newCountingWriter(outFile)

		switch fp.OutputFormat {
		case "zip":
			zw := zip.NewWriter(fp.archiveCounter)
			fp.zipWriter = zw
			fp.archiveWriter = zw
		case "tar.gz":
			gw := gzip.NewWriter(fp.archiveCounter)
			tw := tar.NewWriter(gw)
			fp.tarWriter = tw
			fp.gzipWriter = gw
			fp.archiveWriter = multiCloser{tw, gw}
		}

		defer func() {
			if fp.archiveWriter != nil {
				if cerr := fp.archiveWriter.Close(); cerr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Error closing archive writer: %v\n", cerr)
				}
			}
			if fp.fileWriter != nil {
				if cerr := fp.fileWriter.Close(); cerr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Error closing output file: %v\n", cerr)
				}
			}
		}()
	}

	for _, path := range paths {
		err = fp.processPath(path)
		if err != nil {
			if errors.Is(err, ErrMaxTokensExceeded) {
				_, _ = fmt.Fprintf(os.Stderr, "Reached maximum total tokens limit of %d\n", fp.MaxTokens)
				return nil
			} else if errors.Is(err, ErrMaxTotalSizeExceeded) {
				_, _ = fmt.Fprintf(os.Stderr, "Reached maximum total size limit of %d bytes\n", fp.MaxTotalSize)
				return nil
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "Error processing path %s: %v\n", path, err)
				return err
			}
		}
	}

	if isArchiveOutput && err == nil {
		if fp.archiveWriter != nil {
			if cerr := fp.archiveWriter.Close(); cerr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error closing archive writer after processing: %v\n", cerr)
				return cerr
			}
		}
		if fp.fileWriter != nil {
			if cerr := fp.fileWriter.Close(); cerr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error closing output file after processing: %v\n", cerr)
				return cerr
			}
		}
		fp.archiveWriter = nil
		fp.fileWriter = nil
		fp.zipWriter = nil
		fp.tarWriter = nil
		fp.gzipWriter = nil
		fp.archiveCounter = nil
	}

	return nil
}

func (fp *FileProcessor) processPath(path string) error {
	if fp.MaxTokens > 0 && fp.TotalTokens >= fp.MaxTokens {
		return ErrMaxTokensExceeded
	}
	if fp.MaxTotalSize > 0 && fp.TotalSize >= fp.MaxTotalSize {
		return ErrMaxTotalSizeExceeded
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: Failed to stat path %s: %v\n", path, err)
		return nil
	}

	if fp.Filter != nil && !fp.Filter.FilterPath(path) {
		return nil
	}

	if fileInfo.IsDir() {
		return fp.processDirectory(path)
	} else {
		return fp.processFileContent(path, fileInfo)
	}
}

func (fp *FileProcessor) processDirectory(dirPath string) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("error reading directory %s: %w", dirPath, err)
	}

	dirTokens := 0
	for _, file := range files {
		fullPath := filepath.Join(dirPath, file.Name())

		if fp.Filter != nil && !fp.Filter.FilterPath(fullPath) {
			continue
		}

		err := fp.processPath(fullPath)
		if err != nil {
			return err
		}
		dirTokens += fp.TokenCounts[fullPath]
	}
	fp.TokenCounts[dirPath] = dirTokens
	return nil
}

func (fp *FileProcessor) processFileContent(filePath string, fileInfo os.FileInfo) error {
	if fp.ListOnly {
		fmt.Println(filePath)
		return nil
	}

	var fileStats FileStats
	if fp.Processor != nil {
		var ok bool
		if fp.Stats != nil {
			fileStats, ok = fp.Stats.GetStats(filePath)
		}
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: Stats not found for file %s, continuing without precomputed values\n", filePath)
		}
	}

	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filePath, err)
		return nil
	}

	limitedContent := fp.applyLimits(contentBytes)
	actualSize := int64(len(limitedContent))
	actualTokenCount := len(fp.TokenCounter.Encode(limitedContent, nil, nil))
	actualLineCount := strings.Count(limitedContent, "\n")
	if fileStats.LineCount == 0 {
		fileStats.LineCount = actualLineCount
	}

	fileStats.Size = actualSize
	fileStats.TokenCount = actualTokenCount

	if fp.MaxTotalSize != 0 && fp.TotalSize+actualSize > fp.MaxTotalSize {
		return ErrMaxTotalSizeExceeded
	}
	if fp.MaxTokens != 0 && fp.TotalTokens+actualTokenCount > fp.MaxTokens {
		return ErrMaxTokensExceeded
	}

	fp.TotalSize += actualSize
	fp.TotalTokens += actualTokenCount
	fp.TokenCounts[filePath] = actualTokenCount
	fp.FileCount++

	contentBytesLimited := []byte(limitedContent)
	compressedBefore := fp.currentArchiveSize()

	switch fp.OutputFormat {
	case "zip":
		if fp.zipWriter == nil {
			return fmt.Errorf("internal error: zip writer not initialized for file %s", filePath)
		}
		relativePath := getArchivePath(filePath)
		archivePath := path.Join(fp.ArchivePrefix, relativePath)
		fileWriter, err := fp.zipWriter.Create(archivePath)
		if err != nil {
			return fmt.Errorf("failed to create entry %s in zip archive: %w", archivePath, err)
		}
		_, err = fileWriter.Write(contentBytesLimited)
		if err != nil {
			return fmt.Errorf("failed to write content for %s to zip archive: %w", archivePath, err)
		}

		compressedDelta := fp.bytesWrittenSince(compressedBefore)
		fp.reportArchiveInclusion(archivePath, actualSize, actualLineCount, actualTokenCount, compressedDelta)

	case "tar.gz":
		if fp.tarWriter == nil {
			return fmt.Errorf("internal error: tar writer not initialized for file %s", filePath)
		}
		relativePath := getArchivePath(filePath)
		archivePath := path.Join(fp.ArchivePrefix, relativePath)
		hdr, err := tar.FileInfoHeader(fileInfo, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header for %s: %w", filePath, err)
		}
		hdr.Name = archivePath
		hdr.Size = actualSize
		if err := fp.tarWriter.WriteHeader(hdr); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %w", archivePath, err)
		}
		if _, err := fp.tarWriter.Write(contentBytesLimited); err != nil {
			return fmt.Errorf("failed to write content for %s to tar archive: %w", archivePath, err)
		}

		if fp.gzipWriter != nil {
			if err := fp.gzipWriter.Flush(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to flush gzip writer: %v\n", err)
			}
		}
		compressedDelta := fp.bytesWrittenSince(compressedBefore)
		fp.reportArchiveInclusion(archivePath, actualSize, actualLineCount, actualTokenCount, compressedDelta)

	case "text", "":

		if fp.Processor != nil {
			ctx := context.Background()
			err := fp.Processor.AddRow(ctx, types.NewRow(
				types.MRP("Path", filePath),
				types.MRP("FileSize", fileStats.Size),
				types.MRP("FileTokenCount", fileStats.TokenCount),
				types.MRP("FileLineCount", fileStats.LineCount),
				types.MRP("ActualSize", actualSize),
				types.MRP("ActualTokenCount", actualTokenCount),
				types.MRP("ActualLineCount", actualLineCount),
				types.MRP("Content", limitedContent),
			))
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error adding row to processor: %v\n", err)
			}
		} else {
			switch fp.DelimiterType {
			case "xml":
				fmt.Printf("<file name=\"%s\">\n<content>\n%s\n</content>\n</file>\n", filePath, limitedContent)
			case "markdown":
				fmt.Printf("## File: %s\n\n```\n%s\n```\n\n", filePath, limitedContent)
			case "simple":
				fmt.Printf("--- START FILE: %s ---\n%s\n--- END FILE: %s ---\n", filePath, limitedContent, filePath)
			case "begin-end":
				fmt.Printf("--- BEGIN FILE: %s ---\n%s\n--- END FILE: %s ---\n", filePath, limitedContent, filePath)
			default:
				fmt.Printf("File: %s\n%s\n", filePath, limitedContent)
			}
		}
	default:
		return fmt.Errorf("unknown output format: %s", fp.OutputFormat)
	}

	return nil
}

func getArchivePath(fullPath string) string {
	wd, err := os.Getwd()
	if err != nil {
		return fullPath
	}
	relPath, err := filepath.Rel(wd, fullPath)
	if err != nil {
		return fullPath
	}
	return relPath
}

func (fp *FileProcessor) applyLimits(contentBytes []byte) string {
	content := string(contentBytes)

	if fp.MaxLines > 0 {
		lines := strings.SplitN(content, "\n", fp.MaxLines+1)
		if len(lines) > fp.MaxLines {
			content = strings.Join(lines[:fp.MaxLines], "\n")
		}
	}

	if fp.MaxTokens > 0 {
		tokens := fp.TokenCounter.Encode(content, nil, nil)

		if len(tokens) > fp.MaxTokens {
			truncatedTokens := tokens[:fp.MaxTokens]
			content = fp.TokenCounter.Decode(truncatedTokens)
		}
	}

	return content
}

func (fp *FileProcessor) printConfiguredFilters() {
	fmt.Println("Configured Filters:")
	fmt.Println("-------------------")

	if fp.Filter == nil {
		fmt.Println("No filters configured.")
		return
	}

	fmt.Printf("Max File Size: %d bytes\n", fp.Filter.MaxFileSize)
	fmt.Printf("Disable Default Filters: %v\n", fp.Filter.DisableDefaultFilters)
	fmt.Printf("Disable GitIgnore: %v\n", fp.Filter.DisableGitIgnore)
	fmt.Printf("Filter Binary Files: %v\n", fp.Filter.FilterBinaryFiles)
	fmt.Printf("Verbose: %v\n", fp.Filter.Verbose)

	printStringList("Include Extensions", fp.Filter.IncludeExts)
	printStringList("Exclude Extensions", fp.Filter.ExcludeExts)
	printStringList("Exclude Directories", fp.Filter.ExcludeDirs)

	printRegexpList("Match Filenames", fp.Filter.MatchFilenames)
	printRegexpList("Match Paths", fp.Filter.MatchPaths)
	printRegexpList("Exclude Match Filenames", fp.Filter.ExcludeMatchFilenames)
	printRegexpList("Exclude Match Paths", fp.Filter.ExcludeMatchPaths)

	fmt.Println("\nFile Processor Settings:")
	fmt.Printf("Max Total Size: %d bytes\n", fp.MaxTotalSize)
	fmt.Printf("Max Lines: %d\n", fp.MaxLines)
	fmt.Printf("Max Tokens: %d\n", fp.MaxTokens)
	fmt.Printf("List Only: %v\n", fp.ListOnly)
	fmt.Printf("Delimiter Type: %s\n", fp.DelimiterType)
}

func (fp *FileProcessor) currentArchiveSize() int64 {
	if fp.archiveCounter == nil {
		return 0
	}
	return fp.archiveCounter.BytesWritten()
}

func (fp *FileProcessor) bytesWrittenSince(previous int64) int64 {
	current := fp.currentArchiveSize()
	delta := current - previous
	if delta < 0 {
		return 0
	}
	return delta
}

func (fp *FileProcessor) reportArchiveInclusion(archivePath string, sizeBytes int64, lineCount, tokenCount int, compressedBytes int64) {
	compressionInfo := ""
	if compressedBytes > 0 {
		compressionInfo = fmt.Sprintf(", compressed %d bytes", compressedBytes)
		if compressedBytes != sizeBytes {
			ratio := float64(sizeBytes) / float64(compressedBytes)
			compressionInfo = fmt.Sprintf("%s (%.2fx)", compressionInfo, ratio)
		}
	}

	fmt.Printf(
		"Added to archive: %s (%d bytes, %d lines, %d tokens%s)\n",
		archivePath,
		sizeBytes,
		lineCount,
		tokenCount,
		compressionInfo,
	)
}

func printStringList(name string, list []string) {
	if len(list) > 0 {
		fmt.Printf("%s: %s\n", name, strings.Join(list, ", "))
	}
}

func printRegexpList(name string, list []*regexp.Regexp) {
	if len(list) > 0 {
		patterns := make([]string, len(list))
		for i, re := range list {
			patterns[i] = re.String()
		}
		fmt.Printf("%s: %s\n", name, strings.Join(patterns, ", "))
	}
}

type multiCloser []io.Closer

func (mc multiCloser) Close() error {
	var err error
	for i := len(mc) - 1; i >= 0; i-- {
		if e := mc[i].Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

type countingWriter struct {
	writer  io.Writer
	written int64
}

func newCountingWriter(w io.Writer) *countingWriter {
	return &countingWriter{writer: w}
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.writer.Write(p)
	cw.written += int64(n)
	return n, err
}

func (cw *countingWriter) BytesWritten() int64 {
	return cw.written
}
