# goscan

[![TravisCI Build Status](https://travis-ci.org/joelanford/goscan.svg?branch=master)](https://travis-ci.org/joelanford/goscan)
[![Appveyor Build status](https://ci.appveyor.com/api/projects/status/0gqcc4fkn9dg9lm8/branch/master?svg=true)](https://ci.appveyor.com/project/joelanford/goscan/branch/master)
[![Coverage Status](https://coveralls.io/repos/github/joelanford/goscan/badge.svg?branch=master)](https://coveralls.io/github/joelanford/goscan?branch=master)

goscan is a simple tool to find keywords in text, binary, and archive files.

It copies files to be scanned into a temporary scratch space and recursively walks
directory trees to unarchive, decompress, and scan files with a configurable list 
of keywords.

## Install

`go get -u github.com/joelanford/goscan`

## Using a ramdisk

`goscan` can use a ramdisk to dramatically increase performance for large archives
and compressed files. Using a ramdisk is supported on macOS and Linux, only. Linux
users will need root privileges to use a ramdisk.

By default, ramdisk scratch space is disabled. To enable it, set 
`-scratch.ramdisk.enable=true` on the command line

## Dependencies

### unar

`goscan` also requires the `unar` command line tool

#### CentOS

`sudo yum install -y unar`

#### Debian

`sudo apt-get install -y unar`

#### macOS

`brew install unar`

#### Windows

Download and install from here: [http://unarchiver.c3.cx/downloads/unar1.8.1_win.zip](http://unarchiver.c3.cx/downloads/unar1.8.1_win.zip)

NOTE: You must extract all of the files in the zip into a location on your path.

## Usage

```
Usage: goscan [options] <scanfiles>
  -basedir string
    	Scratch directory for scan unarchiving (default "/tmp/")
  -context int
    	Context to capture around each hit (default 10)
  -hitsonly
    	Only output results containing hits
  -output.file string
    	Results output file ("-" for stdout) (default "-")
  -output.format string
    	Results output format (default "json")
  -parallelism int
    	Number of goroutines to use to scan files (default 8)
  -policies string
    	Comma-separated list of keyword policies (default "all")
  -ramdisk.enable
    	Enable ramdisk scratch directory
  -ramdisk.size int
    	Size of ramdisk (in MB) to use as scratch space (default 4096)
  -words string
    	YAML keywords file
```
