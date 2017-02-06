TODO:

1. Cleanup main.go
2. Handle panic (cleanly unmount ramdisk)
3. Improve this README
4. Add extra hit field for reason
5. Filter words to search for based on blacklist labels


func Scan(file) error {
    hits := dirtywords.Match(file)
    if file.IsArchive() {
        newfiles := file.Unarchive()
        for _, newfile := range newfiles {
            newhits, err := Scan(newfile)
            if err != nil {
                return hits, err
            }
            hits = append(hits, newhits...)
        }
    }
    return hits, nil
}

