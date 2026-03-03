module example.com/thirdparty-pinocchio-tui-poc

go 1.22

require (
	github.com/charmbracelet/bubbletea v1.2.4
	github.com/go-go-golems/bobatea v0.0.0
	github.com/go-go-golems/geppetto v0.0.0
	github.com/go-go-golems/pinocchio v0.0.0
)

replace github.com/go-go-golems/pinocchio => ../../../../../../../
replace github.com/go-go-golems/geppetto => ../../../../../../../../geppetto
replace github.com/go-go-golems/bobatea => ../../../../../../../../bobatea
