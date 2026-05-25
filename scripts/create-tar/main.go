package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: create-tar <archive> <source_root> <root_name> [exec_prefix...]")
		os.Exit(2)
	}

	archivePath := os.Args[1]
	sourceRoot := os.Args[2]
	rootName := cleanTarPath(os.Args[3])
	execPrefixes := make([]string, 0, len(os.Args)-4)
	for _, arg := range os.Args[4:] {
		execPrefixes = append(execPrefixes, cleanTarPath(arg))
	}

	if err := writeArchive(archivePath, sourceRoot, rootName, execPrefixes); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeArchive(archivePath string, sourceRoot string, rootName string, execPrefixes []string) error {
	sourceInfo, err := os.Stat(sourceRoot)
	if err != nil {
		return err
	}

	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	rootHeader := &tar.Header{
		Name:     rootName,
		Typeflag: tar.TypeDir,
		Mode:     0o755,
		ModTime:  sourceInfo.ModTime(),
	}
	if err := tw.WriteHeader(rootHeader); err != nil {
		return err
	}

	paths := make([]string, 0, 128)
	if err := filepath.Walk(sourceRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == sourceRoot {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(paths)

	for _, path := range paths {
		if err := addPath(tw, sourceRoot, rootName, execPrefixes, path); err != nil {
			return err
		}
	}

	return nil
}

func addPath(tw *tar.Writer, sourceRoot string, rootName string, execPrefixes []string, path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(sourceRoot, path)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	name := rootName + "/" + rel

	header := &tar.Header{
		Name:    name,
		ModTime: info.ModTime(),
	}

	if info.IsDir() {
		header.Typeflag = tar.TypeDir
		header.Mode = 0o755
		return tw.WriteHeader(header)
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	header.Typeflag = tar.TypeReg
	header.Size = info.Size()
	header.Mode = fileMode(rel, execPrefixes)
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(tw, file)
	return err
}

func fileMode(rel string, execPrefixes []string) int64 {
	for _, prefix := range execPrefixes {
		if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
			return 0o755
		}
	}
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".sh", ".app":
		return 0o755
	default:
		return 0o644
	}
}

func cleanTarPath(value string) string {
	value = filepath.ToSlash(value)
	value = strings.TrimRight(value, "/")
	if value == "" {
		return "."
	}
	return value
}
