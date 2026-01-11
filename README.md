# speedc

Simple command-line tool to measure internet speed using Cloudflare's speed test infrastructure.

## Features

- Download and upload speed measurement
- Configurable concurrent connections
- Real-time speed display with animation
- Customizable test duration
- Detailed information mode

## Installation

```bash
go install github.com/mattn/speedc@latest
```

Or build locally:

```bash
git clone https://github.com/mattn/speedc
cd speedc
go build -o speedc
```

## Usage

```bash
speedc [options]
```

## Options

- `-concurrent N` - Number of concurrent connections (default: number of CPU cores)
- `-duration N` - Test duration in seconds (default: 5)
- `-noanim` - Disable animation
- `-download-url URL` - Custom download test URL (default: Cloudflare)
- `-upload-url URL` - Custom upload test URL (default: Cloudflare)
- `-i` - Show detailed information

## Examples

Basic speed test:

```bash
speedc
```

Test with 8 concurrent connections for 10 seconds:

```bash
speedc -concurrent 8 -duration 10
```

Test without animation:

```bash
speedc -noanim
```

Show detailed results:

```bash
speedc -i
```

## License

MIT

## Author

Yasuharu YAMASHITA (a.k.a. mattn)
