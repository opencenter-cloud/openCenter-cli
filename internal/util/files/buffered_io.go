/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package files

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// DefaultBufferSize is the default buffer size for file operations
	DefaultBufferSize = 64 * 1024 // 64KB
	
	// LargeBufferSize is used for large file operations
	LargeBufferSize = 256 * 1024 // 256KB
)

// BufferedFileReader provides buffered file reading with optimized I/O
type BufferedFileReader struct {
	file       *os.File
	reader     *bufio.Reader
	bufferSize int
}

// NewBufferedFileReader creates a new buffered file reader
func NewBufferedFileReader(filePath string, bufferSize int) (*BufferedFileReader, error) {
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}
	
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	
	reader := bufio.NewReaderSize(file, bufferSize)
	
	return &BufferedFileReader{
		file:       file,
		reader:     reader,
		bufferSize: bufferSize,
	}, nil
}

// Read reads data into the provided buffer
func (bfr *BufferedFileReader) Read(p []byte) (int, error) {
	return bfr.reader.Read(p)
}

// ReadLine reads a single line from the file
func (bfr *BufferedFileReader) ReadLine() (string, error) {
	line, err := bfr.reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read line: %w", err)
	}
	return line, err
}

// ReadAll reads all content from the file
func (bfr *BufferedFileReader) ReadAll() ([]byte, error) {
	return io.ReadAll(bfr.reader)
}

// Close closes the file reader
func (bfr *BufferedFileReader) Close() error {
	return bfr.file.Close()
}

// BufferedFileWriter provides buffered file writing with optimized I/O
type BufferedFileWriter struct {
	file       *os.File
	writer     *bufio.Writer
	bufferSize int
	filePath   string
}

// NewBufferedFileWriter creates a new buffered file writer
func NewBufferedFileWriter(filePath string, bufferSize int, perm os.FileMode) (*BufferedFileWriter, error) {
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}
	
	if perm == 0 {
		perm = 0644
	}
	
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	
	writer := bufio.NewWriterSize(file, bufferSize)
	
	return &BufferedFileWriter{
		file:       file,
		writer:     writer,
		bufferSize: bufferSize,
		filePath:   filePath,
	}, nil
}

// Write writes data to the file
func (bfw *BufferedFileWriter) Write(p []byte) (int, error) {
	return bfw.writer.Write(p)
}

// WriteString writes a string to the file
func (bfw *BufferedFileWriter) WriteString(s string) (int, error) {
	return bfw.writer.WriteString(s)
}

// Flush flushes the buffer to disk
func (bfw *BufferedFileWriter) Flush() error {
	if err := bfw.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}
	return nil
}

// Sync syncs the file to disk
func (bfw *BufferedFileWriter) Sync() error {
	if err := bfw.Flush(); err != nil {
		return err
	}
	if err := bfw.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}
	return nil
}

// Close closes the file writer (flushes buffer first)
func (bfw *BufferedFileWriter) Close() error {
	if err := bfw.Flush(); err != nil {
		bfw.file.Close()
		return err
	}
	return bfw.file.Close()
}

// ReadFileBuffered reads a file with buffered I/O
func ReadFileBuffered(filePath string) ([]byte, error) {
	reader, err := NewBufferedFileReader(filePath, DefaultBufferSize)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	
	return reader.ReadAll()
}

// WriteFileBuffered writes data to a file with buffered I/O
func WriteFileBuffered(filePath string, data []byte, perm os.FileMode) error {
	writer, err := NewBufferedFileWriter(filePath, DefaultBufferSize, perm)
	if err != nil {
		return err
	}
	defer writer.Close()
	
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}
	
	return writer.Sync()
}

// CopyFileBuffered copies a file with buffered I/O
func CopyFileBuffered(srcPath, dstPath string, bufferSize int) error {
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}
	
	// Open source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer srcFile.Close()
	
	// Get source file info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}
	
	// Create destination file
	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dstPath, err)
	}
	defer dstFile.Close()
	
	// Create buffered reader and writer
	reader := bufio.NewReaderSize(srcFile, bufferSize)
	writer := bufio.NewWriterSize(dstFile, bufferSize)
	
	// Copy data
	if _, err := io.Copy(writer, reader); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}
	
	// Flush and sync
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}
	
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}
	
	return nil
}

// AppendFileBuffered appends data to a file with buffered I/O
func AppendFileBuffered(filePath string, data []byte, perm os.FileMode) error {
	if perm == 0 {
		perm = 0644
	}
	
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	
	// Open file in append mode
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, perm)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()
	
	// Create buffered writer
	writer := bufio.NewWriter(file)
	
	// Write data
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}
	
	// Flush and sync
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}
	
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}
	
	return nil
}

// ReadLinesBuffered reads a file line by line with a callback
func ReadLinesBuffered(filePath string, callback func(line string) error) error {
	reader, err := NewBufferedFileReader(filePath, DefaultBufferSize)
	if err != nil {
		return err
	}
	defer reader.Close()
	
	for {
		line, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read line: %w", err)
		}
		
		if err := callback(line); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}
	}
	
	return nil
}
