package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/pkg/errors"
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
	return fmt.Sprintf("%02d-%02d-%02d", t.Year(), t.Month(), t.Day())
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
	cmd := exec.Command("./image_brightness.py", filepath)
	var outbuf, errbuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &outbuf, &errbuf
	err := cmd.Run()
	if err != nil {
		return 0, errors.Wrap(err, errbuf.String())
	}

	brightnessStr := strings.TrimSpace(outbuf.String())

	brightness, err := strconv.ParseFloat(brightnessStr, 64)
	return brightness, err
}

// @Return filtered and sorted images
func ReadImageFileInfos(imgDir string, excludeDates []time.Time) ImageFileInfos {
	files, err := ioutil.ReadDir(imgDir)
	if err != nil {
		log.Fatal(err)
	}

	var mutex = &sync.Mutex{}
	var imageInfos = make(ImageFileInfos, 0)

	var wg sync.WaitGroup
	wg.Add(len(files))

	processImage := func(file os.FileInfo) {
		defer wg.Done()

		if file.Size() == 0 {
			return
		}

		ext := filepath.Ext(file.Name())
		if ext != ".jpg" {
			return
		}

		tm, err := ImageFileToTimestamp(file.Name())
		if err != nil {
			log.Fatal(err) // TODO
		}

		// TODO: make this not be O(n^2)
		for _, excludedDay := range excludeDates {
			if TimesOnSameDay(tm, excludedDay) {
				// filtered out, we're done here
				return
			}
		}

		log.Printf("calculating brightness %s\n", file.Name())

		fpath := filepath.Join(imgDir, file.Name())
		b, err := CalculateImageBrightness(fpath)
		if err != nil {
			log.Fatal(err)
		}

		info := ImageFileInfo{
			Filename:   file.Name(),
			Timestamp:  tm,
			Brightness: b,
		}

		mutex.Lock()
		imageInfos = append(imageInfos, info)
		mutex.Unlock()
	}

	parallel := false

	if parallel {
		// Process all image files concurrently, adding the results to imageInfos
		for _, file := range files {
			go processImage(file)
		}

		wg.Wait()
	} else {
		for _, file := range files {
			processImage(file)
		}
	}

	sort.Sort(imageInfos)

	return imageInfos
}

// @param imgInfos a *sorted* list of ImageFileInfo objects
func FilterAndGroupByDay(imgInfos ImageFileInfos) []ImageFileInfos {
	grouped := make([]ImageFileInfos, 0)

	for _, info := range imgInfos {
		if info.Brightness < BrightnessThreshold {
			log.Println("Skipping dark file: ", info.Filename)
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
	var outbuf, errbuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &outbuf, &errbuf
	if err := cmd.Run(); err != nil {
		return "", errors.Wrap(err, outbuf.String())
	}

	return outPath, nil
}

func GenerateDailyTimelapses(grouping []ImageFileInfos, imgDir string, outDir string) {
	tmpDir, err := ioutil.TempDir("/tmp", "timelapse")
	if err != nil {
		log.Fatal(err)
	}

	// Create a timelapse for each day in parallel
	var wg sync.WaitGroup
	wg.Add(len(grouping))
	for _, infos := range grouping {
		go func(infos ImageFileInfos) {
			defer wg.Done()
			outPath, err := GenerateTimelapseForImages(infos, tmpDir, imgDir, outDir)
			if err != nil {
				log.Println("Error generating timelapse ", err)
				return
			}

			log.Printf("Created timelapse at '%s'\n", outPath)
		}(infos)
	}
	wg.Wait()
}

// date in form 2017-07-28
func ParseDate(dateStr string) (time.Time, error) {
	layout := "2006-01-02"
	return time.Parse(layout, dateStr)
}

// Get timestamps of timelapses that already exist in the output directory
func DetectExistingTimelapses(outDir string) []time.Time {
	files, err := ioutil.ReadDir(outDir)
	if err != nil {
		log.Fatal(err)
	}

	var videoTimestamps []time.Time

	// Process all image files concurrently, adding the results to imageInfos
	for _, file := range files {
		ext := filepath.Ext(file.Name())
		if ext != ".avi" {
			return nil
		}

		tms := strings.Replace(file.Name(), ext, "", -1)
		t, err := ParseDate(tms)
		if err != nil {
			fmt.Printf("Unable to parse date from timelapse file: %s\n", file.Name())
			continue
		}
		videoTimestamps = append(videoTimestamps, t)
	}

	return videoTimestamps
}

func main() {
	imgDir := flag.String("image-dir", "./", "Directory of timestamped image files")
	outDir := flag.String("out-dir", "", "Directory to store completed timelapses and html. This directory will be served publicly, so don't put anything secret in here.")
	httpPort := flag.String("port", "8888", "Port to serve on")
	updateInterval := flag.Int("update-interval", 60*60, "How often, in seconds, to regenerate all timelapses.")
	flag.Parse()

	if *outDir == "" {
		fmt.Fprintf(os.Stderr, "Please specify an output directory\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	updateAll := func() {
		log.Printf("Detecting existing timelapses...\n")
		existing := DetectExistingTimelapses(*outDir)
		var excludeDates []time.Time
		if len(existing) > 0 {
			excludeDates = existing[:len(existing)-1]
		} else {
			excludeDates = existing
		}

		log.Printf("=> Done detecting timelapses. Found %d\n", len(existing))

		log.Printf("Reading image directory...\n")
		imageInfos := ReadImageFileInfos(*imgDir, excludeDates)
		log.Printf("=> Found %d images\n", len(imageInfos))
		grouped := FilterAndGroupByDay(imageInfos)

		log.Printf("Generating %d timelapses...\n", len(grouped))
		GenerateDailyTimelapses(grouped, *imgDir, *outDir)
	}

	go updateAll()

	// update on a fixed interval
	interval := time.Duration(*updateInterval) * time.Second
	go func() {
		for _ = range time.Tick(interval) {
			log.Printf("It's been %d seconds, updating timelapses...\n", *updateInterval)
			updateAll()
		}
	}()

	// periodically regenerate timelapse videos
	go log.Printf("Serving '%s' on port '%s'\n", *outDir, *httpPort)
	http.Handle("/", http.FileServer(http.Dir(*outDir)))
	log.Fatal(http.ListenAndServe(":"+*httpPort, nil))
}
