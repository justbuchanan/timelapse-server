package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Images with brightness below this level are filtered out
const BrightnessThreshold = 0.25

func ParseTimestamp(timestamp string) (time.Time, error) {
	i, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	tm := time.Unix(i, 0)
	return tm, nil
}

func FormatDate(t time.Time) string {
	return fmt.Sprintf("%d-%d-%d", t.Year(), t.Month(), t.Day())
}

func ImageFileToTimestamp(fname string) (time.Time, error) {
	tms := strings.Replace(fname, "-image.jpg", "", -1)
	t, err := ParseTimestamp(tms)
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}

func TimesOnSameDay(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.Month() == t2.Month() && t1.Day() == t2.Day()
}

type ImageFileInfo struct {
	Filename   string
	Timestamp  time.Time
	Brightness float64
}

func (info ImageFileInfo) String() string {
	return fmt.Sprintf("%s, %s, %0.2f", info.Filename, info.Timestamp.String(), info.Brightness)
}

/* Sort slices of ImageFileInfo objects */

type ImageFileInfos []ImageFileInfo

func (slice ImageFileInfos) Len() int {
	return len(slice)
}

func (slice ImageFileInfos) Less(i, j int) bool {
	return slice[j].Timestamp.After(slice[i].Timestamp)
}

func (slice ImageFileInfos) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func CalculateImageBrightness(filepath string) (float64, error) {
	// run python script to calculate image brightness
	b, err := exec.Command("./image_brightness.py", filepath).Output()
	if err != nil {
		return 0, err
	}

	brightnessStr := strings.TrimSpace(string(b))

	brightness, err := strconv.ParseFloat(brightnessStr, 64)
	return brightness, err
}

// @Return filtered and sorted images
func ReadImageFileInfos(imgDir string) ImageFileInfos {
	files, err := ioutil.ReadDir(imgDir)
	if err != nil {
		log.Fatal(err)
	}

	var mutex = &sync.Mutex{}
	var imageInfos = make(ImageFileInfos, 0)

	var wg sync.WaitGroup
	wg.Add(len(files))

	// Process all image files concurrently, adding the results to imageInfos
	for _, file := range files {
		go func(file os.FileInfo) {
			defer wg.Done()

			if file.Size() == 0 {
				return
			}

			ext := filepath.Ext(file.Name())
			if ext != ".jpg" {
				return
			}

			fpath := filepath.Join(imgDir, file.Name())
			b, err := CalculateImageBrightness(fpath)
			if err != nil {
				log.Fatal(err)
			}

			tm, err := ImageFileToTimestamp(file.Name())
			if err != nil {
				log.Fatal(err) // TODO
			}

			info := ImageFileInfo{
				Filename:   file.Name(),
				Timestamp:  tm,
				Brightness: b,
			}

			mutex.Lock()
			imageInfos = append(imageInfos, info)
			mutex.Unlock()
		}(file)
	}

	wg.Wait()

	sort.Sort(imageInfos)

	return imageInfos
}

// @param imgInfos a *sorted* list of ImageFileInfo objects
func FilterAndGroupByDay(imgInfos ImageFileInfos) []ImageFileInfos {
	grouped := make([]ImageFileInfos, 0)

	for _, info := range imgInfos {
		if info.Brightness < BrightnessThreshold {
			continue
		}

		if len(grouped) > 0 {
			prevDayGroup := grouped[len(grouped)-1]
			prev := prevDayGroup[len(prevDayGroup)-1]

			if TimesOnSameDay(prev.Timestamp, info.Timestamp) {
				// append to the existing day group
				prevDayGroup = append(prevDayGroup, info)
				grouped[len(grouped)-1] = prevDayGroup

				continue
			}
		}

		// TODO: do better?
		// new day
		dayGroup := make(ImageFileInfos, 0)
		dayGroup = append(dayGroup, info)
		grouped = append(grouped, dayGroup)
	}

	return grouped
}

func GenerateTimelapseForImages(images ImageFileInfos, tmpDir string, imgDir string, outDir string) (string, error) {
	firstImg := images[0]
	dayStr := FormatDate(firstImg.Timestamp)
	manifestFilepath := filepath.Join(tmpDir, dayStr+".txt")

	// open manifest file
	manifest, err := os.Create(manifestFilepath)
	if err != nil {
		return "", err
	}

	for _, img := range images {
		imgFilepath, err := filepath.Abs(filepath.Join(imgDir, img.Filename))
		if err != nil {
			return "", err
		}
		manifest.WriteString(imgFilepath)
		manifest.WriteString("\n")
	}
	manifest.Close()

	// use mencoder create timelapse video in output directory
	outPath := filepath.Join(outDir, dayStr+".avi")
	fps := 20
	cmd := exec.Command("mencoder", "-nosound", "-ovc", "lavc", "-mf", fmt.Sprintf("type=jpeg:fps=%d", fps), fmt.Sprintf("mf://@%s", manifestFilepath), "-o", outPath)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return outPath, nil
}

func GenerateDailyTimelapses(grouping []ImageFileInfos, imgDir string, outDir string) {
	tmpDir, err := ioutil.TempDir("/tmp", "timelapse")
	if err != nil {
		log.Fatal(err)
	}

	for _, infos := range grouping {
		outPath, err := GenerateTimelapseForImages(infos, tmpDir, imgDir, outDir)
		if err != nil {
			log.Println("Error generating timelapse ", err)
		} else {
			log.Printf("Created timelapse at '%s'\n", outPath)
		}
	}
}

func main() {
	imgDir := flag.String("image-dir", "./", "Directory of timestamped image files")
	outDir := flag.String("out-dir", "", "Directory to store completed timelapses and html. This directory will be served publicly, so don't put anything secret in here.")
	httpPort := flag.String("port", "8888", "Port to serve on")
	flag.Parse()

	if *outDir == "" {
		fmt.Fprintf(os.Stderr, "Please specify an output directory\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// periodically regenerate timelapse videos
	updateInterval := 20 * time.Second
	go func() {
		for _ = range time.Tick(updateInterval) {
			imageInfos := ReadImageFileInfos(*imgDir)
			log.Printf("Found %d images\n", len(imageInfos))
			grouped := FilterAndGroupByDay(imageInfos)
			GenerateDailyTimelapses(grouped, *imgDir, *outDir)
		}
	}()

	log.Printf("Serving '%s' on port '%s'\n", *outDir, *httpPort)
	http.Handle("/", http.FileServer(http.Dir(*outDir)))
	log.Fatal(http.ListenAndServe(":"+*httpPort, nil))
}
