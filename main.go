package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

/*
Header is as follows:

4 bytes | 0x00 to 0x03 | Magic
4 bytes | 0x04 to 0x07 | Data Size (in bytes)
4 bytes | 0x08 to 0x0B | Sample Rate (48kHz default)
4 bytes | 0x0C to 0x0F | Channels (1 or 2, not sure if it can be higher)
4 bytes | 0x10 to 0x13 | Bit Depth (16, not sure if it can be higher)
12 bytes | 0x14 to 0x1f | Reserved

32 bytes in total
*/

type SMPHeader struct {
	Magic      [4]byte /* treat as 4 bytes, RFWV, same 4 chars */
	DataSize   uint32  /* interpret as uint32, as 4 bytes unsigned can only be positive */
	SampleRate uint32
	Channels   uint32
	BitDepth   uint32
}

type WAVHeader struct {
	ChunkID       [4]byte
	ChunkSize     uint32
	Format        [4]byte
	Subchunk1ID   [4]byte
	Subchunk1Size uint32
	AudioFormat   uint16
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
	Subchunk2ID   [4]byte
	Subchunk2Size uint32
}

func main() {
	inputDir := flag.String("i", "", "input directory containing .smp files")
	outputDir := flag.String("o", "", "output directory for .wav files (defaults to input directory)")
	flag.Parse()

	filenames, err := resolveInput(*inputDir, flag.Args())
	if err != nil {
		log.Fatal(err)
	}

	if len(filenames) == 0 {
		fmt.Println("Usage: smp2wav <file.smp> [file2.smp ...]  OR  smp2wav -i <file or dir> [-o <dir>]")
		os.Exit(1)
	}

	processFiles(filenames, *outputDir)
}

func resolveInput(inputFlag string, positionalArgs []string) ([]string, error) {
	var filenames []string

	if inputFlag != "" {
		info, err := os.Stat(inputFlag)
		if err != nil {
			return nil, err
		}

		if info.IsDir() {
			entries, err := os.ReadDir(inputFlag)
			if err != nil {
				return nil, err
			}

			for _, entry := range entries {
				if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".smp") {
					filenames = append(filenames, filepath.Join(inputFlag, entry.Name()))
				}
			}
		} else {
			filenames = append(filenames, inputFlag)
		}
	} else {
		filenames = positionalArgs
	}

	return filenames, nil
}

func processFiles(filenames []string, outputDir string) {
	successCount := 0
	failCount := 0

	for _, filename := range filenames {
		if !strings.EqualFold(filepath.Ext(filename), ".smp") {
			fmt.Println("skipping:", filename)
			continue
		}

		if err := convertFile(filename, outputDir); err != nil {
			fmt.Println("error converting", filename, ":", err)
			failCount++
			continue
		}

		fmt.Println("converted:", filename)
		successCount++
	}

	fmt.Printf("\nDone, %d converted, %d failed.\n", successCount, failCount)
}

func parseSMPHeader(buf []byte) (SMPHeader, error) {
	var header SMPHeader

	copy(header.Magic[:], buf[0:4])

	if header.Magic != [4]byte{'R', 'F', 'W', 'V'} {
		return header, fmt.Errorf("not a valid SMP file: bad magic bytes")
	}

	header.DataSize = binary.BigEndian.Uint32(buf[4:8])
	header.SampleRate = binary.BigEndian.Uint32(buf[8:12])
	header.Channels = binary.BigEndian.Uint32(buf[12:16])
	header.BitDepth = binary.BigEndian.Uint32(buf[16:20])

	fmt.Printf("Magic: %s, DataSize: %d, SampleRate: %d, Channels: %d, BitDepth: %d\n",
		header.Magic, header.DataSize, header.SampleRate, header.Channels, header.BitDepth)

	return header, nil
}

func buildWAVHeader(header SMPHeader) []byte {
	byteRate := header.SampleRate * header.Channels * (header.BitDepth / 8)
	blockAlign := header.Channels * (header.BitDepth / 8)
	chunkSize := 36 + header.DataSize

	wavBuf := make([]byte, 44)

	copy(wavBuf[0:4], []byte("RIFF"))                                     /* ChunkID */
	binary.LittleEndian.PutUint32(wavBuf[4:8], chunkSize)                 /* ChunkSize */
	copy(wavBuf[8:12], []byte("WAVE"))                                    /* Format */
	copy(wavBuf[12:16], []byte("fmt "))                                   /* Subchunk1ID */
	binary.LittleEndian.PutUint32(wavBuf[16:20], 16)                      /* Subchunk1Size, always 16 for PCM */
	binary.LittleEndian.PutUint16(wavBuf[20:22], 1)                       /* AudioFormat, 1 is PCM */
	binary.LittleEndian.PutUint16(wavBuf[22:24], uint16(header.Channels)) /* NumChannels */
	binary.LittleEndian.PutUint32(wavBuf[24:28], header.SampleRate)       /* SampleRate */
	binary.LittleEndian.PutUint32(wavBuf[28:32], byteRate)                /* ByteRate */
	binary.LittleEndian.PutUint16(wavBuf[32:34], uint16(blockAlign))      /* BlockAlign */
	binary.LittleEndian.PutUint16(wavBuf[34:36], uint16(header.BitDepth)) /* BitsPerSample */
	copy(wavBuf[36:40], []byte("data"))                                   /* Subchunk2ID */
	binary.LittleEndian.PutUint32(wavBuf[40:44], header.DataSize)         /* Subchunk2Size */

	return wavBuf
}

func swapByteOrder(data []byte) {
	for i := 0; i < len(data); i += 2 {
		data[i], data[i+1] = data[i+1], data[i]
	}
}

func convertFile(filename string, outputDir string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := make([]byte, 32)
	n, err := file.Read(buf)
	if err != nil {
		return err
	}
	if n != 32 {
		return fmt.Errorf("file too short to contain a valid header")
	}

	header, err := parseSMPHeader(buf)
	if err != nil {
		return err
	}

	pcmData := make([]byte, header.DataSize)
	n, err = file.Read(pcmData)
	if err != nil {
		return err
	}
	if n != len(pcmData) {
		return fmt.Errorf("expected %d bytes of PCM data, got %d", len(pcmData), n)
	}

	swapByteOrder(pcmData)

	wavBuf := buildWAVHeader(header)

	baseName := filepath.Base(filename)
	outName := strings.TrimSuffix(baseName, filepath.Ext(baseName)) + ".wav"

	var outPath string
	if outputDir != "" {
		outPath = filepath.Join(outputDir, outName)
	} else {
		outPath = filepath.Join(filepath.Dir(filename), outName)
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	outFile.Write(wavBuf)
	outFile.Write(pcmData)

	return nil
}
