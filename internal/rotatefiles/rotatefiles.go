package rotatefiles

import (
	"fmt"
	"log"
	"os"
	"sort"
)

// Rotate files: keep <num> of most recent files and delete other
func RotateFilesByMtime(filesDir string, filesToKeep int) error {
	fileToDel := ""

	files, err := os.ReadDir(filesDir)
	if err != nil {
		return err
	}

	// sort file slice by modification time(asc)
	sort.Slice(files, func(i, j int) bool {
		fileI, err := files[i].Info()
		if err != nil {
			log.Fatal(err)
		}

		fileJ, err := files[j].Info()
		if err != nil {
			log.Fatal(err)
		}

		return fileI.ModTime().After(fileJ.ModTime())
	})

	// delete files which index more than <filesToKeep> value
	for ind, file := range files {
		// skip dir
		if file.IsDir() {
			continue
		}

		// deleting old files
		if ind+1 > filesToKeep {
			fileToDel = fmt.Sprintf("%s/%s", filesDir, file.Name())

			if err := os.Remove(fileToDel); err != nil {
				log.Printf("failed to remove file %s:\n\t%v\n", file.Name(), err)
			} else {
				log.Printf("file %s removed\n", file.Name())
			}
		}
	}

	return nil
}
