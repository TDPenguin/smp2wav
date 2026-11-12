# smp2wav

A small CLI tool to convert Roland SP-404 MKII `.SMP` sample export files into `.wav` files.

## Background

The SP-404 MKII's "export" feature produces `.SMP` files rather than standard WAV files. This tool parses the SMP header, extracts the PCM audio data, and rewrites it as a playable 16-bit PCM WAV file.

### SMP file format

32-byte header, big-endian:

| Offset | Size | Field | Notes |
|---|---|---|---|
| 0x00 | 4 bytes | Magic | Always `RFWV` |
| 0x04 | 4 bytes | Data Size | Length of PCM data in bytes |
| 0x08 | 4 bytes | Sample Rate | e.g. 48000 |
| 0x0C | 4 bytes | Channels | e.g. 2 (stereo) |
| 0x10 | 4 bytes | Bit Depth | e.g. 16 |
| 0x14 | 12 bytes | Reserved | Unused/reserve |

PCM audio data follows.

## Usage

Convert one or more specific files:
```bash
go run main.go file1.SMP file2.SMP
```

Convert an entire folder:
```bash
go run main.go -i Input/
```

Convert a folder, writing output elsewhere:
```bash
go run main.go -i Input/ -o Output/
```

By default, output `.wav` files are written alongside their source `.SMP` files, unless `-o` is given.

## Requirements

- Go 1.27+