# goscan

`goscan` is a simple tool to find keywords in text, binary, and archive files. 

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

## Usage

```
Usage: goscan [options] <scanfiles>
  -scan.context int
    	Context to capture around each hit (default 10)
  -scan.output string
    	Results output file ("-" for stdout) (default "-")
  -scan.parallelism int
    	Number of goroutines to use to scan files. (default 8)
  -scan.policies string
    	Comma-separated list of keyword policies (default "all")
  -scan.words string
    	YAML keywords file
  -scratch.basedir string
    	Scratch directory for scan unarchiving (default "/tmp/")
  -scratch.ramdisk.enable
    	Enable ramdisk scratch directory
  -scratch.ramdisk.mb int
    	Size of ramdisk to use as scratch space (default 4096)
```
