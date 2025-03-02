# MADROOKBOT CODE GUIDE

## Build & Test Commands
- Build: `go build -mod=vendor -o madrookbot`
- Run: `./madrookbot`
- Run single test: `go test -v -run=ExampleSheetGeneratorGenerateOne` 
- Run all tests: `go test -v ./...`

## Code Style Guidelines
- **Formatting**: Standard Go formatting (gofmt)
- **Imports**: Group standard library first, then third-party packages
- **Naming**: 
  - CamelCase for exported functions/variables/types
  - lowerCamelCase for unexported functions/variables/types
- **Error Handling**: Check errors immediately, return early pattern
- **Structs**: Define fields with descriptive names, group related fields
- **Documentation**: Add comments for exported functions/types

## Project Structure
- **Main files**: Single package architecture
- **Dependencies**: Uses vendored dependencies
- **Environment**: Uses environment variables for configuration