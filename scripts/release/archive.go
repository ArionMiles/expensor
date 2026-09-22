package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type archiveEntry struct {
	name string
	path string
	mode int64
}

var targets = []string{"linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64"}

func main() {
	input := flag.String("input", "", "directory containing target binaries")
	output := flag.String("output", "", "directory for release archives")
	version := flag.String("version", "", "release version")
	config := flag.String("config", "", "example configuration path")
	license := flag.String("license", "", "license path")
	notice := flag.String("notice", "", "notice path")
	flag.Parse()

	if err := packageArchives(*input, *output, *version, *config, *license, *notice); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func packageArchives(input, output, version, config, license, notice string) error {
	if input == "" || output == "" || version == "" || config == "" || license == "" || notice == "" {
		return fmt.Errorf("all archive arguments are required")
	}
	if strings.ContainsAny(version, `/\\`) || version == "." || version == ".." {
		return fmt.Errorf("invalid version %q", version)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	checksums := make([]string, 0, len(targets))
	for _, target := range targets {
		name := fmt.Sprintf("expensor_%s_%s.tar.gz", version, target)
		entries := []archiveEntry{
			{name: "LICENSE", path: license, mode: 0o644},
			{name: "NOTICE", path: notice, mode: 0o644},
			{name: "config.toml.example", path: config, mode: 0o644},
			{name: "expensor", path: filepath.Join(input, "expensor_"+target), mode: 0o755},
		}
		archivePath := filepath.Join(output, name)
		if err := writeArchive(archivePath, entries); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		digest, err := fileChecksum(archivePath)
		if err != nil {
			return fmt.Errorf("checksum %s: %w", name, err)
		}
		checksums = append(checksums, fmt.Sprintf("%x  %s\n", digest, name))
	}

	manifest := filepath.Join(output, fmt.Sprintf("expensor_%s_checksums.txt", version))
	if err := os.WriteFile(manifest, []byte(strings.Join(checksums, "")), 0o644); err != nil {
		return fmt.Errorf("write checksum manifest: %w", err)
	}
	return nil
}

func writeArchive(destination string, entries []archiveEntry) (returnErr error) {
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); returnErr == nil {
			returnErr = err
		}
	}()

	compressed := gzip.NewWriter(file)
	compressed.Header.ModTime = time.Unix(0, 0).UTC()
	compressed.Header.OS = 255
	archive := tar.NewWriter(compressed)
	for _, entry := range entries {
		contents, err := os.ReadFile(entry.path)
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.path, err)
		}
		header := &tar.Header{
			Name: entry.name, Mode: entry.mode, Size: int64(len(contents)),
			ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatUSTAR,
		}
		if err := archive.WriteHeader(header); err != nil {
			return err
		}
		if _, err := archive.Write(contents); err != nil {
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	return compressed.Close()
}

func fileChecksum(path string) ([sha256.Size]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return [sha256.Size]byte{}, err
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}
