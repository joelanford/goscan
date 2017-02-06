# goscan

`goscan` is a simple tool to find keywords in text, binary, and archive files. 

It temporarily mounts a ramdisk for scratch space and recursively walks directory trees
to unarchive, decompress, and scan files with a configurable list of keywords.

NOTE: `goscan` is currently only supported on macOS due to OS-specific ramdisk setup processes. 
A future release will relax the requirement for ramdisk and add support for other OSes

## Install

`go get -u github.com/joelanford/goscan`

## Dependencies

### unar

`goscan` also requires the `unar` command line tool

#### CentOS

`sudo yum install -y unar`

#### Debian

`sudo apt-get install -y unar`

#### macOS

`brew install unar`

## Usage

```
Usage of ./goscan:
  -ramdisk.basedir string
    	Base directory for ramdisk mountpoints (default "/tmp")
  -ramdisk.megabytes int
    	Size of ramdisk to use as scratch space (default 4096)
  -ramdisk.name string
    	Disk label to use for ramdisk (default "goscan")
  -scan.db string
    	Database to track previously seen files (default "/Users/joe/goscan.sqlite")
  -scan.results string
    	Results output file ("-" for stdout) (default "-")
  -scan.words string
    	YAML dirty words file
```
