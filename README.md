[![rcard](https://goreportcard.com/badge/github.com/kubec/chia-log-analyzer)](https://goreportcard.com/report/github.com/kubec/chia-log-analyzer)
[![License](https://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://raw.githubusercontent.com/kubec/chia-log-analyzer/master/LICENSE)

# Chia log analyzer
Simply realtime chia log analyzer

## Howto run
```
chia-log-analyzer.go-linux-amd64 --log=/path/to/debug.log
```
or simply copy binary file to the directory with logs and run without parameters

### debug.log locations
Automatically trying to load debug.log from these locations:
* ./debug.log (actual directory)
* get log path from the parameter **"--log"**
* ~/.chia/mainnet/log/debug.log (default directory in home dir)

![Screenshot](./docs/screenshot-1.png)

## Howto install
Download binary from the [releases](../../releases) assets

## Features
- monitoring of chia debug.log file
- simply show basic info about farming
- automatic refresh every 5s

## Supported platforms
- Linux (tested on Ubuntu) - download binary: **chia-log-analyzer.go-linux-amd64**
- RPI4 (use linux-arm builds) - download binary:  **chia-log-analyzer.go-linux-arm**
- Windows10 - download binary:  **chia-log-analyzer.go-windows-amd64.exe**

## Keys
- **q** - exit

## Donations
Thank you...

**Chia coins (XCH)** - xch16agqsnzhrf55x0f4f7y8k0kq9xz6rvh99nfd86cc3lnse8kgn5qs5y6ywn

**Bitcoin** - 3GvUQUPPbp396jYoZsAMktgg5XWE9g6con