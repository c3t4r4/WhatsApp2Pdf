# Windows 64-bit
GOOS=windows GOARCH=amd64 go build -o whats2pdf.exe main.go 

# macOS Intel/AMD
GOOS=darwin GOARCH=amd64 go build -o whats2pdf-mac main.go && chmod +x whats2pdf-mac

# macOS Apple Silicon (M1/M2)
GOOS=darwin GOARCH=arm64 go build -o whats2pdf-mac-arm main.go && chmod +x whats2pdf-mac-arm

# Linux 64-bit
GOOS=linux GOARCH=amd64 go build -o whats2pdf-linux main.go && chmod +x whats2pdf-linux