package main

import "archive/zip"
import "bufio"
import "encoding/json"
import "flag"
import "fmt"
import "html/template"
import "image"
import _ "image/jpeg"
import "io"
import "log"
import "net/http"
import "os"
import "path/filepath"
import "regexp"
import "strconv"
import "strings"

var version string

type Config struct {
	webDashboardUrl string
	apiUsername     string
	apiToken        string
	companyId       string
	sessionId       string
	orientation     string
}

var outDir string

func main() {
	fmt.Printf("Version: %s\n", version)

	var webDashboardUrl = flag.String("web-dashboard-url", "", "")
	var apiUsername = flag.String("api-username", "", "")
	var apiToken = flag.String("api-token", "", "")
	var companyId = flag.String("company-id", "", "")
	var sessionId = flag.String("session-id", "", "")
	var orientation = flag.String("orientation", "landscape", "")
	var port = flag.String("port", "3333", "")

	flag.Parse()

	if *webDashboardUrl == "" {
		log.Fatalln("Web Dashboard URL must be specified")
	}

	if *apiUsername == "" {
		log.Fatalln("API username must be specified")
	}

	if *apiToken == "" {
		log.Fatalln("API token must be specified")
	}

	if *companyId == "" {
		log.Fatalln("Company ID must be specified")
	}

	if *sessionId == "" {
		log.Fatalln("Session ID must be specified")
	}

	config := Config{
		webDashboardUrl: *webDashboardUrl,
		apiUsername:     *apiUsername,
		apiToken:        *apiToken,
		companyId:       *companyId,
		sessionId:       *sessionId,
		orientation:     *orientation,
	}

	firstMetricTimestamp := lookupSession(config)
	zipFile := downloadSession(config)
	outDir = unzipSession(config.sessionId, zipFile)
	err := os.Remove(zipFile)
	if err != nil {
		log.Fatalln(err)
	}

	screenshots := listScreenshots(outDir)
	logLines := getLogLines(0, 500)

	outputPath := generateHtml(firstMetricTimestamp, screenshots, logLines, config.orientation, *port)

	fmt.Printf("Please open %s in your browser\n", outputPath)

	http.HandleFunc("/", getRoot)
	http.HandleFunc("/logs", getLogs)

	fmt.Printf("Server listening on 127.0.0.1:%s\n", *port)

	err = http.ListenAndServe(fmt.Sprintf("127.0.0.1:%s", *port), nil)
	log.Fatalln(err)
}

func getRoot(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Pong\n")
}

func getLogs(w http.ResponseWriter, r *http.Request) {
	from, err := strconv.Atoi(r.URL.Query().Get("from"))
	if err != nil {
		log.Fatalln(err)
	}
	to, err := strconv.Atoi(r.URL.Query().Get("to"))
	if err != nil {
		log.Fatalln(err)
	}

	logLines := getLogLines(from, to)
	logs := processLogLines(logLines)

	encoded, _ := json.Marshal(logs)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	io.WriteString(w, string(encoded))
}

type SessionResponse struct {
	MinAbsTSCharts uint64 `json:"minAbsTSCharts"`
}

func lookupSession(config Config) uint64 {
	client := &http.Client{
		Transport: &http.Transport{},
	}

	url := fmt.Sprintf("%s/v1/sessions/%s?company=%s", config.webDashboardUrl, config.sessionId, config.companyId)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.SetBasicAuth(config.apiUsername, config.apiToken)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		log.Fatalln("Session not found")
	}

	sessionResponse := &SessionResponse{}

	json.NewDecoder(resp.Body).Decode(sessionResponse)

	return sessionResponse.MinAbsTSCharts
}

func downloadSession(config Config) string {
	client := &http.Client{
		Transport: &http.Transport{},
	}

	url := fmt.Sprintf("%s/v1/sessions/export/sessions/%s?company=%s", config.webDashboardUrl, config.sessionId, config.companyId)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.SetBasicAuth(config.apiUsername, config.apiToken)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		log.Fatalln("Session not found")
	}

	filename := fmt.Sprintf("%s.zip", config.sessionId)

	zipFile, err := os.Create(filename)
	if err != nil {
		log.Fatalln(err)
	}

	defer zipFile.Close()

	io.Copy(zipFile, resp.Body)

	return filename
}

func unzipSession(sessionId string, zipFile string) string {
	f, err := os.MkdirTemp("", "session-export")
	if err != nil {
		log.Fatalln(err)
	}

	unzip(zipFile, f)

	files, err := filepath.Glob(fmt.Sprintf("%s/**/*.zip", f))
	if err != nil {
		log.Fatalln(err)
	}

	outDir := fmt.Sprintf("./sessions/%s", sessionId)

	unzip(files[0], outDir)

	return outDir
}

func getLogLines(from int, to int) []string {
	files, err := filepath.Glob(fmt.Sprintf("%s/**/**/logcat.txt", outDir))
	if err != nil {
		log.Fatalln(err)
	}

	if len(files) == 0 {
		files, err = filepath.Glob(fmt.Sprintf("%s/**/**/android_app_logcat.txt", outDir))
		if err != nil {
			log.Fatalln(err)
		}
	}

	if len(files) == 0 {
		log.Fatalln("Log file not found")
	}

	path := files[0]

	file, err := os.Open(path)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	n := 0

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		n++

		if n < from {
			continue
		}

		if n > to {
			break
		}

		lines = append(lines, scanner.Text())
	}

	return lines
}

func listScreenshots(sessionDir string) []string {
	files, err := filepath.Glob(fmt.Sprintf("%s/**/**/fbsnapshots/*.jpg", sessionDir))
	if err != nil {
		log.Fatalln(err)
	}

	return files
}

type Screenshot struct {
	Path            string
	Timestamp       uint64
	PrettyTimestamp uint64
}

type LogEntry struct {
	Second uint64 `json:"second"`
	Entry  string `json:"entry"`
	First  bool   `json:"first"`
}

var firstHours *uint64
var firstMinutes *uint64
var firstSeconds *uint64
var prev uint64
var first bool

func processLogLines(logLines []string) []*LogEntry {
	logs := make([]*LogEntry, 0)

	r2 := regexp.MustCompile(`^(?:[0-9]{4}-[0-9]{2}-[0-9]{2} )?([0-9]{2}):([0-9]{2}):([0-9]{2})\.`)
	r3 := regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2} `)

	for _, logLine := range logLines {
		matches := r2.FindStringSubmatch(logLine)
		if len(matches) == 0 {
			continue
		}

		dateMatches := r3.MatchString(logLine)

		if dateMatches == true && (firstHours == nil || firstMinutes == nil) {
			parsed, err := strconv.ParseUint(matches[1], 10, 64)
			if err != nil {
				log.Fatalln(err)
			}

			firstHours = &parsed

			minutesParsed, err := strconv.ParseUint(matches[2], 10, 64)
			if err != nil {
				log.Fatalln(err)
			}

			firstMinutes = &minutesParsed

			secondsParsed, err := strconv.ParseUint(matches[3], 10, 64)
			if err != nil {
				log.Fatalln(err)
			}

			firstSeconds = &secondsParsed
		}

		hours, err := strconv.ParseUint(matches[1], 10, 64)
		if err != nil {
			log.Fatalln(err)
		}

		minutes, err := strconv.ParseUint(matches[2], 10, 64)
		if err != nil {
			log.Fatalln(err)
		}

		seconds, err := strconv.ParseUint(matches[3], 10, 64)
		if err != nil {
			log.Fatalln(err)
		}

		if firstHours != nil {
			hours = hours - *firstHours
		}

		if firstMinutes != nil {
			minutes = minutes - *firstMinutes
		}

		if firstSeconds != nil {
			seconds = seconds - *firstSeconds
		}

		total := seconds + (minutes * 60) + (hours * 60 * 60)

		if prev != total {
			first = true
			prev = total
		} else {
			first = false
		}

		logs = append(logs, &LogEntry{
			Entry:  logLine,
			Second: total,
			First:  first,
		})
	}

	return logs
}

func generateHtml(firstMetricTimestamp uint64, screenshotPaths []string, logLines []string, orientation string, port string) string {
	// Parse the HTML template from a file.
	tmpl, err := template.ParseFiles("template.html")
	if err != nil {
		log.Fatalln(err)
	}

	// Open a new file for writing.
	output, err := os.Create("output.html")
	if err != nil {
		log.Fatalln(err)
	}
	defer output.Close()

	screenshots := make([]*Screenshot, 0)

	r := regexp.MustCompile(`/([0-9]+)\.jpg$`)

	var screenshotWidth *int
	var screenshotHeight *int

	for _, screenshot := range screenshotPaths {
		if screenshotWidth == nil || screenshotHeight == nil {
			returnedScreenshotWidth, returnedScreenshotHeight := getImageDimensions(screenshot)
			screenshotWidth = &returnedScreenshotWidth
			screenshotHeight = &returnedScreenshotHeight
		}

		matches := r.FindStringSubmatch(screenshot)
		timestampStr := matches[1]

		timestamp, err := strconv.ParseUint(timestampStr, 10, 64)
		if err != nil {
			log.Fatalln(err)
		}

		screenshots = append(screenshots, &Screenshot{
			Path:            screenshot,
			Timestamp:       timestamp - firstMetricTimestamp,
			PrettyTimestamp: (timestamp - firstMetricTimestamp),
		})
	}

	logs := processLogLines(logLines)

	if *screenshotWidth > *screenshotHeight {
		orientation = "landscape"
	} else {
		orientation = "portrait"
	}

	// Execute the template and write the output to the file.
	data := struct {
		Screenshots      []*Screenshot
		LogLines         []*LogEntry
		Orientation      string
		ScreenshotWidth  int
		ScreenshotHeight int
		Port             string
	}{
		Screenshots:      screenshots,
		LogLines:         logs,
		Orientation:      orientation,
		ScreenshotWidth:  *screenshotWidth,
		ScreenshotHeight: *screenshotHeight,
		Port:             port,
	}

	err = tmpl.Execute(output, data)
	if err != nil {
		log.Fatalln(err)
	}

	absPath, err := filepath.Abs("output.html")
	if err != nil {
		log.Fatalln(err)
	}

	return absPath
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	os.MkdirAll(dest, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				log.Fatalln(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					log.Fatalln(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func getImageDimensions(imagePath string) (int, int) {
	file, err := os.Open(imagePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	image, _, err := image.DecodeConfig(file)
	if err != nil {
		log.Fatalln(err)
	}

	return image.Width, image.Height
}
