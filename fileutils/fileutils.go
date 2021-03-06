// This file is part of go-utils.
//
// Copyright (C) 2015-2016  David Gamba Rios
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

/*
Package fileutils - file related utilities
*/
package fileutils

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// StringError is a struct containing the string `String` and error `Error`.
type StringError struct {
	String string
	Error  error
}

// Internal struct used to hold the basedname of a file.
// Used for sorting purposes.
type fileParts struct {
	full string
	base string
}

// byName implements sort.Interface.
type byName []os.FileInfo

func (f byName) Len() int      { return len(f) }
func (f byName) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f byName) Less(i, j int) bool {
	nai, err := strconv.Atoi(f[i].Name())
	if err != nil {
		return f[i].Name() < f[j].Name()
	}
	naj, err := strconv.Atoi(f[j].Name())
	if err != nil {
		return f[i].Name() < f[j].Name()
	}
	return nai < naj
}

type byBase []fileParts

func (a byBase) Len() int      { return len(a) }
func (a byBase) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byBase) Less(i, j int) bool {
	nai, err := strconv.Atoi(a[i].base)
	if err != nil {
		return a[i].base < a[j].base
	}
	naj, err := strconv.Atoi(a[j].base)
	if err != nil {
		return a[i].base < a[j].base
	}
	return nai < naj
}

// SortSameDirFilesNumerically - sorts a list of files in the same dir (they all have the same dirname) numerically.
// The files are only sorted numerically when all element basenames are numbers.
func SortSameDirFilesNumerically(fileList []string, reverse bool) []string {
	var files []fileParts
	for _, e := range fileList {
		fp := fileParts{e, filepath.Base(e)}
		files = append(files, fp)
	}
	if reverse {
		sort.Sort(sort.Reverse(byBase(files)))
	} else {
		sort.Sort(byBase(files))
	}
	var sortedFileList []string
	for _, e := range files {
		sortedFileList = append(sortedFileList, e.full)
	}
	return sortedFileList
}

// CopyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	err = out.Sync()
	return err
}

// GetFileList returns a channel with each file (`channel.String`) or an error indicating failure (`channel.Error`).
func GetFileList(dirname string, ignoreDirs, recursive bool) <-chan StringError {
	c := make(chan StringError)
	go func() {
		fInfo, err := os.Stat(dirname)
		if err != nil {
			c <- StringError{"", err}
			close(c)
			return
		}
		if fInfo.IsDir() {
			fileSearch := dirname + string(filepath.Separator) + "*"
			fileMatches, err := filepath.Glob(fileSearch)
			if err != nil {
				c <- StringError{"", err}
				close(c)
				return
			}
			for _, file := range fileMatches {
				fInfo, err := os.Stat(file)
				if err != nil {
					c <- StringError{"", err}
					continue
				}
				if fInfo.IsDir() {
					if ignoreDirs == false {
						c <- StringError{file, nil}
					}
					if recursive {
						d := GetFileList(file, ignoreDirs, recursive)
						for dirFile := range d {
							c <- dirFile
						}
					}
				} else {
					c <- StringError{file, nil}
				}
			}
		} else {
			c <- StringError{"", fmt.Errorf("Provided dir is not a dir: '%s'\n", dirname)}
			close(c)
			return
		}
		close(c)
	}()
	return c
}

// ListFiles returns []string with a list of files.
func ListFiles(dirname string, ignoreDirs, recursive bool) ([]string, error) {
	files := []string{}
	fInfo, err := os.Stat(dirname)
	if err != nil {
		return nil, err
	}
	if fInfo.Mode()&os.ModeSymlink != 0 {
		rl, err := os.Readlink(dirname)
		if err != nil {
			return nil, err
		}
		fInfo, err = os.Stat(rl)
		if err != nil {
			return nil, err
		}
	}
	if fInfo.IsDir() {
		fileMatches, err := ioutil.ReadDir(dirname)
		if err != nil {
			return nil, err
		}
		for _, file := range fileMatches {
			var rlFile os.FileInfo
			if file.Mode()&os.ModeSymlink != 0 {
				rl, err := os.Readlink(dirname + string(os.PathSeparator) + file.Name())
				if err != nil {
					return files, err
				}
				if !filepath.IsAbs(rl) {
					rl = dirname + string(os.PathSeparator) + rl
				}
				rlFile, err = os.Stat(rl)
				if err != nil {
					return nil, err
				}
			}
			if (rlFile != nil && rlFile.IsDir()) || file.IsDir() {
				if !ignoreDirs {
					files = append(files, dirname+string(os.PathSeparator)+file.Name())
				}
				if recursive {
					fl, err := ListFiles(dirname+string(os.PathSeparator)+file.Name(), ignoreDirs, recursive)
					if err != nil {
						return files, err
					}
					files = append(files, fl...)
				}
			} else {
				files = append(files, dirname+string(os.PathSeparator)+file.Name())
			}
		}
	} else {
		return nil, fmt.Errorf("Provided dir is not a dir: '%s'\n", dirname)
	}
	return files, nil
}

// ReadDirNumSort - Same as ioutil/ReadDir but uses returns a Numerically
// Sorted file list.
//
// Taken from https://golang.org/src/io/ioutil/ioutil.go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Modified Sort method to use Numerically sorted names instead.
// It also allows reverse sorting.
func ReadDirNumSort(dirname string, reverse bool) ([]os.FileInfo, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	if reverse {
		sort.Sort(sort.Reverse(byName(list)))
	} else {
		sort.Sort(byName(list))
	}
	return list, nil
}

// ListFilesNumSort returns []string with a numerically sorted list of files.
func ListFilesNumSort(dirname string, ignoreDirs, recursive, reverse bool) ([]string, error) {
	files := []string{}
	fInfo, err := os.Stat(dirname)
	if err != nil {
		return nil, err
	}
	if fInfo.IsDir() {
		fileSearch := dirname
		fileMatches, err := ReadDirNumSort(fileSearch, reverse)
		if err != nil {
			return nil, err
		}
		for _, file := range fileMatches {
			if file.IsDir() {
				if ignoreDirs == false {
					files = append(files, dirname+string(os.PathSeparator)+file.Name())
				}
				if recursive {
					fl, err := ListFilesNumSort(dirname+string(os.PathSeparator)+file.Name(), ignoreDirs, recursive, reverse)
					if err != nil {
						return files, err
					}
					files = append(files, fl...)
				}
			} else {
				files = append(files, dirname+string(os.PathSeparator)+file.Name())
			}
		}
	} else {
		return nil, fmt.Errorf("Provided dir is not a dir: '%s'\n", dirname)
	}
	return files, nil
}

// GetNumSortFileList - Get Numerically Sorted File List.
// Returns a channel with each file (`channel.String`) or an error indicating failure (`channel.Error`).
func GetNumSortFileList(dirname string, ignoreDirs, recursive, reverse bool) <-chan StringError {
	c := make(chan StringError)
	go func() {
		fInfo, err := os.Stat(dirname)
		if err != nil {
			c <- StringError{"", err}
			close(c)
			return
		}
		if fInfo.IsDir() {
			fileSearch := dirname + string(filepath.Separator) + "*"
			fileMatches, err := filepath.Glob(fileSearch)
			if err != nil {
				c <- StringError{"", err}
				close(c)
				return
			}
			fileMatches = SortSameDirFilesNumerically(fileMatches, reverse)
			for _, file := range fileMatches {
				fInfo, err := os.Stat(file)
				if err != nil {
					c <- StringError{"", err}
					continue
				}
				if fInfo.IsDir() {
					if ignoreDirs == false {
						c <- StringError{file, nil}
					}
					if recursive {
						d := GetNumSortFileList(file, ignoreDirs, recursive, reverse)
						for dirFile := range d {
							c <- dirFile
						}
					}
				} else {
					c <- StringError{file, nil}
				}
			}
		} else {
			c <- StringError{"", fmt.Errorf("Provided dir is not a dir: '%s'\n", dirname)}
			close(c)
			return
		}
		close(c)
	}()
	return c
}

// GetDirList returns a channel with each file (`channel.String`) or an error indicating failure (`channel.Error`).
func GetDirList(dirname string) <-chan StringError {
	c := make(chan StringError)
	go func() {
		fInfo, err := os.Stat(dirname)
		if err != nil {
			c <- StringError{"", err}
			close(c)
			return
		}
		if fInfo.IsDir() {
			fileSearch := dirname + string(filepath.Separator) + "*"
			fileMatches, err := filepath.Glob(fileSearch)
			if err != nil {
				c <- StringError{"", err}
				close(c)
				return
			}
			for _, file := range fileMatches {
				fInfo, err := os.Stat(file)
				if err != nil {
					c <- StringError{"", err}
					continue
				}
				if fInfo.IsDir() {
					c <- StringError{file, nil}
					d := GetDirList(file)
					for dirFile := range d {
						c <- dirFile
					}
				}
			}
		} else {
			c <- StringError{"", fmt.Errorf("Provided dir is not a dir: '%s'\n", dirname)}
			close(c)
			return
		}
		close(c)
	}()
	return c
}

// GetNumSortDirList returns a channel with each file (`channel.String`) or an error indicating failure (`channel.Error`).
func GetNumSortDirList(dirname string, reverse bool) <-chan StringError {
	c := make(chan StringError)
	go func() {
		fInfo, err := os.Stat(dirname)
		if err != nil {
			c <- StringError{"", err}
			close(c)
			return
		}
		if fInfo.IsDir() {
			fileSearch := dirname + string(filepath.Separator) + "*"
			fileMatches, err := filepath.Glob(fileSearch)
			if err != nil {
				c <- StringError{"", err}
				close(c)
				return
			}
			fileMatches = SortSameDirFilesNumerically(fileMatches, reverse)
			for _, file := range fileMatches {
				fInfo, err := os.Stat(file)
				if err != nil {
					c <- StringError{"", err}
					continue
				}
				if fInfo.IsDir() {
					c <- StringError{file, nil}
					d := GetNumSortDirList(file, reverse)
					for dirFile := range d {
						c <- dirFile
					}
				}
			}
		} else {
			c <- StringError{"", fmt.Errorf("Provided dir is not a dir: '%s'\n", dirname)}
			close(c)
			return
		}
		close(c)
	}()
	return c
}

// StringReplace - Runs strings.Replace on each line of the file.
// The file is read line by line to account for large files.
// The changes are first written to a tmp copy is saved before overwriting the
// original. The original is only changed if linesChanged > 0.
func StringReplace(file, old, new string, n, bufferSize int) (int, error) {
	var tmpFile *os.File
	linesChanged := 0
	tmpFile, err := ioutil.TempFile("", filepath.Base(file)+"-")
	if err != nil {
		return 0, fmt.Errorf("cannot open '%s': %s\n", tmpFile.Name(), err)
	}
	defer tmpFile.Close()
	for d := range ReadLines(file, bufferSize) {
		if d.Error != nil {
			return 0, fmt.Errorf("Error reading file '%s': %s\n", file, d.Error)
		}
		line := strings.Replace(d.String, old, new, n)
		if d.String != line {
			linesChanged++
		}
		tmpFile.WriteString(line + "\n")
	}
	tmpFile.Close()
	if linesChanged > 0 {
		err = CopyFile(tmpFile.Name(), file)
		if err != nil {
			return 0, fmt.Errorf("Couldn't update file: %s. '%s'\n", file, err)
		}
	}
	os.Remove(tmpFile.Name())
	return linesChanged, nil
}

// ReadLines - returns a channel of type StringError with each line of a file.
func ReadLines(filename string, bufferSize int) <-chan StringError {
	c := make(chan StringError)
	go func() {
		file, err := os.Open(filename)
		if err != nil {
			c <- StringError{"", fmt.Errorf("Couldn't open file '%s': %s\n", filename, err)}
			close(c)
			return
		}
		defer file.Close()

		reader := bufio.NewReaderSize(file, bufferSize)
		// line number
		n := 0
		for {
			n++
			line, isPrefix, err := reader.ReadLine()
			if isPrefix {
				c <- StringError{"", fmt.Errorf("%s: buffer size too small\n", filename)}
				break
			}
			// stop reading file
			if err != nil {
				if err != io.EOF {
					c <- StringError{"", fmt.Errorf("Read error '%s': %s\n", filename, err)}
				}
				break
			}
			c <- StringError{string(line), nil}
		}
		close(c)
	}()
	return c
}
