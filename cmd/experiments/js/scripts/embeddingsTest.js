async function runEmbeddingsTest() {
    console.log("=== Running Embeddings Test ===");

    // Test model info
    const model = embeddings.getModel();
    console.log("Model info:", model);

    // Test synchronous embedding generation
    const text = "Hello, world!";
    try {
        const embedding = embeddings.generateEmbedding(text);
        const truncatedEmbedding = embedding.slice(0, 10);
        console.log("Generated embedding (first 10 values):", truncatedEmbedding);
        console.log("Embedding dimensions:", embedding.length);
    } catch (err) {
        console.error("Sync embedding generation failed:", err);
    }

    // Test async embedding generation
    try {
        const asyncEmbedding = await embeddings.generateEmbeddingAsync(text);
        const truncatedAsyncEmbedding = asyncEmbedding.slice(0, 10);
        console.log("Generated async embedding (first 10 values):", truncatedAsyncEmbedding);
        console.log("Async embedding dimensions:", asyncEmbedding.length);
    } catch (err) {
        console.error("Async embedding generation failed:", err);
    }

    // Test callback-based embedding generation
    const cancel = embeddings.generateEmbeddingWithCallbacks(text, {
        onSuccess: (embedding) => {
            console.log("Callback embedding dimensions:", embedding.length);
        },
        onError: (err) => {
            console.error("Callback embedding failed:", err);
        }
    });

    // Test semantic similarity example
    const documents = [
        "The weather is sunny today",
        "Machine learning is fascinating",
        "I love programming in JavaScript"
    ];

    function cosineSimilarity(a, b) {
        let dotProduct = 0;
        let normA = 0;
        let normB = 0;

        for (let i = 0; i < a.length; i++) {
            dotProduct += a[i] * b[i];
            normA += a[i] * a[i];
            normB += b[i] * b[i];
        }

        return dotProduct / (Math.sqrt(normA) * Math.sqrt(normB));
    }

    try {
        // Generate embeddings for all documents
        const documentEmbeddings = documents.map(doc =>
            embeddings.generateEmbedding(doc)
        );

        // Generate query embedding
        const query = "What's the weather like?";
        const queryEmbedding = embeddings.generateEmbedding(query);

        // Find most similar document
        const similarities = documentEmbeddings.map(docEmb =>
            cosineSimilarity(queryEmbedding, docEmb)
        );

        const mostSimilarIndex = similarities.indexOf(Math.max(...similarities));
        console.log("Query:", query);
        console.log("Most similar document:", documents[mostSimilarIndex]);
        console.log("Similarity score:", similarities[mostSimilarIndex]);

    } catch (err) {
        console.error("Semantic search failed:", err);
        done(err); // Signal error
        return;
    }

    console.log("Embeddings test complete");
    done(); // Signal completion
}

console.log("Starting Embeddings Test");
runEmbeddingsTest().catch(err => {
    console.error("Test failed:", err);
    done(err); // Signal error
}); 