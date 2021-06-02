// Copyright 2021 Michal Kubec <michal.kubec@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT license that can
// be found in the LICENSE file.

// +build ignore

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	log "github.com/sirupsen/logrus"
)

var debuglogFile *string

var widgetLastTimestamp *widgets.Paragraph
var widgetLastPlots *widgets.Paragraph
var widgetFoundProofs *widgets.Paragraph
var widgetLastFarmingTime *widgets.Paragraph
var widgetTotalFarmingPlotsNumber *widgets.Paragraph
var widgetLog *widgets.Paragraph
var widgetBarChart *widgets.BarChart
var widgetBarChartParagraph *widgets.Paragraph
var widgetBarChart2 *widgets.Plot
var widgetBarChart2Paragraph *widgets.Paragraph
var widgetSparklines *widgets.Sparkline
var widgetSparklinesGroup *widgets.SparklineGroup
var widgetOverallHealthPercent *widgets.Paragraph
var lastRow string = ""
var lastParsedLines []string

var farmingPlotsNumber = 0
var totalFarmingPlotsNumber = "0"

var totalFarmingAttempt = 0
var positiveFarmingAttempt = 0

var foundProofs = 0
var farmingTime = "0"
var totalPlots = "0"
var minFarmingTime = 999999.0
var maxFarmingTime = 0.0
var allFarmingTimes []float64

var lastLogFileSize = int64(0)
var healthData = make(map[string]float64)

type stackStruct struct {
	lines []string
	count int
}

type stackStructFloats struct {
	values []float64
	count  int
}

// keep only X last lines in buffer
func (stack *stackStruct) push(line string) {
	stack.lines = append(stack.lines, line)
	if len(stack.lines) > stack.count {
		stack.lines = stack.lines[1 : stack.count+1]
	}
}

// keep only X last values in buffer
func (stack *stackStructFloats) push(value float64) {
	stack.values = append(stack.values, value)
	if len(stack.values) > stack.count {
		stack.values = stack.values[1 : stack.count+1]
	}
}

var lastParsedLinesStack = stackStruct{count: 5}
var lastFarmStack = stackStructFloats{count: 29}
var lastFarmingTimesStack = stackStructFloats{count: 113}

func main() {

	initLogging()
	detectLogFileLocation()

	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	var smallWidgetHeight = 3

	widgetLastPlots = widgets.NewParagraph()
	widgetLastPlots.SetRect(0, 0, 9, smallWidgetHeight)
	widgetLastPlots.Title = "Plots"
	ui.Render(widgetLastPlots)

	widgetFoundProofs = widgets.NewParagraph()
	widgetFoundProofs.SetRect(9, 0, 18, smallWidgetHeight)
	widgetFoundProofs.Title = "Proofs"
	ui.Render(widgetFoundProofs)

	widgetTotalFarmingPlotsNumber = widgets.NewParagraph()
	widgetTotalFarmingPlotsNumber.SetRect(18, 0, 37, smallWidgetHeight)
	widgetTotalFarmingPlotsNumber.Title = "Farming attempts"
	ui.Render(widgetTotalFarmingPlotsNumber)

	widgetLastFarmingTime = widgets.NewParagraph()
	widgetLastFarmingTime.SetRect(37, 0, 77, smallWidgetHeight)
	widgetLastFarmingTime.Title = "Farming times (last/min/avg/max)"
	ui.Render(widgetLastFarmingTime)

	widgetOverallHealthPercent = widgets.NewParagraph()
	widgetOverallHealthPercent.Title = "Overall farming health indicator"
	widgetOverallHealthPercent.SetRect(77, 0, 119, smallWidgetHeight)
	widgetOverallHealthPercent.TextStyle.Fg = ui.ColorCyan
	widgetOverallHealthPercent.Text = "?? %"

	widgetLog = widgets.NewParagraph()
	widgetLog.SetRect(0, 15, 119, smallWidgetHeight)
	widgetLog.Title = "Last farming"
	ui.Render(widgetLog)

	widgetBarChart = widgets.NewBarChart()
	widgetBarChart.Title = "Plots eligible for farming - last 59 values"
	widgetBarChart.SetRect(0, 15, 119, 25)
	widgetBarChart.BarWidth = 3
	widgetBarChart.BarGap = 1
	widgetBarChart.BarColors = []ui.Color{ui.ColorGreen}
	widgetBarChart.LabelStyles = []ui.Style{ui.NewStyle(ui.ColorBlue)}
	widgetBarChart.NumStyles = []ui.Style{ui.NewStyle(ui.ColorWhite)}

	//widget for "not enough data"
	widgetBarChartParagraph = widgets.NewParagraph()
	widgetBarChartParagraph.SetRect(0, 15, 119, 25) //same as above
	widgetBarChartParagraph.Title = "Not engough data or zero values"

	widgetBarChart2 = widgets.NewPlot()
	widgetBarChart2.Title = "Farming times (axis Y in seconds) - last 113 values"
	widgetBarChart2.Data = make([][]float64, 1)
	widgetBarChart2.SetRect(0, 25, 119, 35)
	widgetBarChart2.AxesColor = ui.ColorWhite
	widgetBarChart2.LineColors[0] = ui.ColorRed
	widgetBarChart2.Marker = widgets.MarkerBraille

	//widget for "not enough data"
	widgetBarChart2Paragraph = widgets.NewParagraph()
	widgetBarChart2Paragraph.SetRect(0, 25, 119, 35) //same as above
	widgetBarChart2Paragraph.Title = "Not engough data or zero values"

	widgetSparklines = widgets.NewSparkline()
	widgetSparklines.Title = "1 col = 10 minutes block. It could be about 64 reqs/ 10minutes. Chart could be almost flat (+/-1 height block)"
	widgetSparklines.LineColor = ui.ColorBlue
	widgetSparklines.TitleStyle.Fg = ui.ColorWhite

	widgetSparklinesGroup = widgets.NewSparklineGroup(widgetSparklines)
	widgetSparklinesGroup.Title = "Health indicator - number of incoming farming requests from the Chia network"
	widgetSparklinesGroup.SetRect(0, 35, 119, 45)

	go loopReadFile()

	uiEvents := ui.PollEvents()
	for {
		e := <-uiEvents
		switch e.ID {
		case "q", "<C-c>":
			return
		}
	}

}

func initLogging() {
	writeLog := flag.Bool("writelog", false, "write log?")
	flag.Parse()
	if *writeLog {
		// If the file doesn't exist, create it or append to the file
		file, err := os.OpenFile("chia-log-analyzer.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(file)
	} else {
		log.SetOutput(ioutil.Discard)
	}

	log.SetLevel(log.InfoLevel)
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	log.Info("Application start")
}

func detectLogFileLocation() {
	debuglogFile = flag.String("log", "./debug.log", "path to debug.log")
	flag.Parse()
	missingLogFile := false

	//1 - try open debug.log in the actual directory or get file from the paremeter "log"
	log.Info(fmt.Sprintf("Trying to open: %s", *debuglogFile))
	if _, err := os.Stat(*debuglogFile); os.IsNotExist(err) {
		missingLogFile = true
	} else {
		return
	}

	//2 - try open debug log from the default home location
	usr, _ := user.Current()
	dir := usr.HomeDir
	defaultLogLocation := fmt.Sprintf("%s/.chia/mainnet/log/debug.log", dir)
	debuglogFile = &defaultLogLocation
	log.Info(fmt.Sprintf("Trying to open: %s", *debuglogFile))
	if _, err := os.Stat(*debuglogFile); os.IsNotExist(err) {
		missingLogFile = true
	} else {
		return
	}

	if missingLogFile == true {
		fmt.Println("Please specify path to the log file, with parameter: log (--log=/path/to/debug.log)")
		os.Exit(0)
	}
}

func loopReadFile() {
	renderLog(fmt.Sprintf("Reading log %s, please wait", *debuglogFile))
	readFullFile(*debuglogFile)
	setLastLogFileSize(*debuglogFile)

	c := time.Tick(5 * time.Second)
	var actualLogFileSize int64
	for range c {
		actualLogFileSize, _ = getFileSize(*debuglogFile)
		log.Info(fmt.Sprintf("Actual log file size: %d bytes", actualLogFileSize))
		if actualLogFileSize == 0 || actualLogFileSize == lastLogFileSize {
			log.Info("No file change, skipping parsing")
			continue
		}
		if actualLogFileSize < lastLogFileSize { // new file ?
			log.Info("New file detected")
			readFullFile(*debuglogFile)
		} else {
			readFile(*debuglogFile)
		}

		setLastLogFileSize(*debuglogFile)
	}
}

func setLastLogFileSize(filepath string) {
	lastLogFileSize, _ = getFileSize(filepath)
	log.Info(fmt.Sprintf("Last file size: %d bytes", lastLogFileSize))
}

func getFileSize(filepath string) (int64, error) {
	fi, err := os.Stat(filepath)
	if err != nil {
		return 0, err
	}
	// get the size
	return fi.Size(), nil
}

func readFullFile(fname string) {
	if _, err := os.Stat(fname); os.IsNotExist(err) {
		return
	}

	log.Info("Reading full file")
	lastRow = ""

	// os.Open() opens specific file in
	// read-only mode and this return
	// a pointer of type os.
	file, err := os.Open(fname)

	if err != nil {
		log.Error(fmt.Sprintf("Failed to open log file: %s, skipping reading", fname))
		return
	}
	defer file.Close()

	// The bufio.NewScanner() function is called in which the
	// object os.File passed as its parameter and this returns a
	// object bufio.Scanner which is further used on the
	// bufio.Scanner.Split() method.
	scanner := bufio.NewScanner(file)

	// The bufio.ScanLines is used as an
	// input to the method bufio.Scanner.Split()
	// and then the scanning forwards to each
	// new line using the bufio.Scanner.Scan()
	// method.
	scanner.Split(bufio.ScanLines)
	var text []string

	for scanner.Scan() {
		text = append(text, scanner.Text())
	}

	parseLines(text)

	// The method os.File.Close() is called
	// on the os.File object to close the file
	file.Close()
}

func readFile(fname string) {
	if _, err := os.Stat(fname); os.IsNotExist(err) {
		log.Error(fmt.Sprintf("Failed to open log file: %s, skipping reading", fname))
		return
	}

	file, err := os.Open(fname)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to open log file: %s, skipping reading", err))
		return
	}
	defer file.Close()

	bytesToRead := 16384

	buf := make([]byte, bytesToRead)
	stat, err := os.Stat(fname)
	start := int64(0)

	//is file big enough ?
	if stat.Size() >= int64(bytesToRead) {
		start = stat.Size() - int64(bytesToRead)
	}

	log.Info(fmt.Sprintf("Reading file. %d bytes from the position: %d", bytesToRead, start))

	_, err = file.ReadAt(buf, start)
	if err == nil {
		lines := strings.Split(string(buf), "\n")
		parseLines(lines)
	}

	file.Close()
}

func parseLines(lines []string) {
	log.Info(fmt.Sprintf("Number of rows for parsing: %d", len(lines)))

	//0 plots were eligible for farming 27274481c3... Found 0 proofs. Time: 0.14447 s. Total 105 plots
	regexPlotsFarming, _ := regexp.Compile("([0-9]+)\\s+plots\\s+were\\seligible.*Found\\s([0-9])+\\sproofs.*Time:\\s([0-9]+\\.[0-9]+)\\ss\\.\\sTotal\\s([0-9]+)\\splots")

	startParsingLines := false
	for i, s := range lines {
		if i == 0 { //skip the first row - can be uncomplete due reading by bytes
			continue
		}
		if s == "" {
			continue
		}
		if !startParsingLines {
			if lastRow == "" { //first run ?
				startParsingLines = true
			} else {
				if lastRow == s {
					startParsingLines = true
				}
				continue
			}
		}

		lastRow = s

		if regexPlotsFarming.MatchString(s) {
			lastParsedLinesStack.push(s)

			//extract number of requests in 10minutes blocks
			runes := []rune(s)
			dateTime := string(runes[0:15]) //1 or 10-minutes (15=10min, 16=1min)
			_, exists := healthData[dateTime]
			if !exists {
				healthData[dateTime] = 0
			}
			healthData[dateTime] = healthData[dateTime] + 1

			//plots + proofs
			match := regexPlotsFarming.FindStringSubmatch(s)
			farmingPlotsNumber, _ := strconv.Atoi(match[1])
			foundProofsActual, _ := strconv.Atoi(match[2])
			if foundProofsActual > 0 {
				foundProofs = foundProofs + foundProofsActual
			}
			farmingTime = match[3]
			totalPlots = match[4]

			if farmingPlotsNumber > 0 {
				positiveFarmingAttempt = positiveFarmingAttempt + farmingPlotsNumber
			}
			totalFarmingAttempt++
			lastFarmStack.push(float64(farmingPlotsNumber)) //data for barchart

			parsedTime, _ := strconv.ParseFloat(farmingTime, 8) //last time

			allFarmingTimes = append(allFarmingTimes, parsedTime) //for AVG computing

			if parsedTime < float64(minFarmingTime) {
				minFarmingTime = parsedTime
			}
			if parsedTime > float64(maxFarmingTime) {
				maxFarmingTime = parsedTime
			}
			lastFarmingTimesStack.push(parsedTime)
		}
	}

	log.Info("Log parsing done")

	renderWidgets()
	renderLastFarmBarChart()
	renderLastFarmBarChart2()
	renderSparkLines()

	var tmpTxt strings.Builder
	for i := range lastParsedLinesStack.lines {
		twoCols := strings.Split(lastParsedLinesStack.lines[i], "    ")
		tmpTxt.WriteString(twoCols[0])
		tmpTxt.WriteString("\n")
		tmpTxt.WriteString("-->")
		tmpTxt.WriteString(twoCols[1])
		tmpTxt.WriteString("\n")
	}
	renderLog(tmpTxt.String())
}

func renderWidgets() {
	renderOverallHealth()
	renderTotalFarmingPlotsNumber()
	renderLastPlots()
	renderFoundProofs()
	renderLastFarmingTime()
}

func renderTotalFarmingPlotsNumber() {
	percent := 0.0
	if totalFarmingAttempt > 0 {
		percent = float64(float64(positiveFarmingAttempt)/float64(totalFarmingAttempt)) * 100
	}
	widgetTotalFarmingPlotsNumber.Text = fmt.Sprintf("%d/%d(%.1f%%)", positiveFarmingAttempt, totalFarmingAttempt, percent)
	ui.Render(widgetTotalFarmingPlotsNumber)
}

func renderLastPlots() {
	widgetLastPlots.Text = fmt.Sprintf("%s", totalPlots)
	ui.Render(widgetLastPlots)
}

func renderFoundProofs() {
	widgetFoundProofs.Text = fmt.Sprintf("%d", foundProofs)
	ui.Render(widgetFoundProofs)
}

func renderLastFarmingTime() {
	total := 0.0
	for _, number := range allFarmingTimes {
		total = total + number
	}
	average := total / float64(len(allFarmingTimes))

	widgetLastFarmingTime.Text = fmt.Sprintf("%ss / %.3fs / %.3fs / %.3fs", farmingTime, minFarmingTime, average, maxFarmingTime)
	ui.Render(widgetLastFarmingTime)
}

func renderLog(text string) {
	widgetLog.Text = fmt.Sprintf("%s", text)
	ui.Render(widgetLog)
}

func renderOverallHealth() {
	if len(healthData) < 3 {
		return
	}
	values := sortMap(healthData)
	values = values[1 : len(values)-1] //remove the first and the last (may be incomplete)
	sum := sumFloats(values)
	avg := sum / float64(len(values))

	//in chia network we can see 6.4 req/minute (64 req/10minutes) for farming
	percent := avg / 64 * 100

	/*
		if percent > 100 { //result may be > 100%, but it is due some meassure inaccuracy
			percent = 100 //overwrite down to 100% is OK
		}*/

	widgetOverallHealthPercent.TextStyle.Fg = ui.ColorRed
	if percent > 80 {
		widgetOverallHealthPercent.TextStyle.Fg = ui.ColorCyan
	}
	if percent > 95 {
		widgetOverallHealthPercent.TextStyle.Fg = ui.ColorGreen
	}
	widgetOverallHealthPercent.Text = fmt.Sprintf("%.2f%%  - normal value is about 100%%", percent)
	ui.Render(widgetOverallHealthPercent)
}

func renderLastFarmBarChart() {
	for _, x := range lastFarmStack.values {
		if x > 0 { //at least one positive value
			widgetBarChart.Data = lastFarmStack.values
			ui.Render(widgetBarChart)
			return
		}
	}

	ui.Render(widgetBarChartParagraph)
}

func renderLastFarmBarChart2() {
	for _, x := range lastFarmingTimesStack.values {
		if x > 0 { //at least one positive value
			widgetBarChart2.Data[0] = lastFarmingTimesStack.values
			ui.Render(widgetBarChart2)
			return
		}
	}

	ui.Render(widgetBarChart2Paragraph)
}

func renderSparkLines() {

	if len(healthData) == 0 {
		return
	}

	v := sortMap(healthData)

	sliceNumberOfValues := 117
	if len(v) < sliceNumberOfValues {
		sliceNumberOfValues = len(v)
		v = v[len(v)-sliceNumberOfValues:]
	}

	widgetSparklines.Data = v
	ui.Render(widgetSparklinesGroup)
}

func sortMap(m map[string]float64) []float64 {
	if len(m) == 0 {
		return make([]float64, 0)
	}

	v := make([]float64, 0, len(m))

	//sort map by keys
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v = append(v, m[k])
	}

	return v
}

func sumFloats(input []float64) float64 {
	sum := 0.0

	for i := range input {
		sum += input[i]
	}

	return sum
}
