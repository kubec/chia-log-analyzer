// Copyright 2021 Michal Kubec <michal.kubec@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT license that can
// be found in the LICENSE file.

// +build ignore

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

var debuglogFile *string

var widgetLastTimestamp *widgets.Paragraph
var widgetLastPlots *widgets.Paragraph
var widgetFoundProofs *widgets.Paragraph
var widgetLastFarmingTime *widgets.Paragraph
var widgetTotalFarmingPlotsNumber *widgets.Paragraph
var widgetLog *widgets.Paragraph
var widgetMinFarmingTime *widgets.Paragraph
var widgetMaxFarmingTime *widgets.Paragraph
var widgetBarChart *widgets.BarChart
var widgetBarChart2 *widgets.Plot
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

var lastLogFileSize = int64(0)

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
var lastFarmingTimesStack = stackStructFloats{count: 110}

func main() {
	debuglogFile = flag.String("log", "./debug.log", "path to debug.log")
	flag.Parse()
	if _, err := os.Stat(*debuglogFile); os.IsNotExist(err) {
		fmt.Println("Please specify path to the log file, with parameter: log (--log=/path/to/debug.log)")
		return
	}

	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	var smallWidgetWidth = 20
	var smallWidgetHeight = 3

	widgetLastPlots = widgets.NewParagraph()
	widgetLastPlots.SetRect(smallWidgetWidth*0, 0, smallWidgetWidth-1, smallWidgetHeight)
	widgetLastPlots.Title = "Last plots count"
	ui.Render(widgetLastPlots)

	widgetFoundProofs = widgets.NewParagraph()
	widgetFoundProofs.SetRect(smallWidgetWidth*1, 0, (smallWidgetWidth*2)-1, smallWidgetHeight)
	widgetFoundProofs.Title = "Found proofs"
	ui.Render(widgetFoundProofs)

	widgetLastFarmingTime = widgets.NewParagraph()
	widgetLastFarmingTime.SetRect(smallWidgetWidth*2, 0, (smallWidgetWidth*3)-1, smallWidgetHeight)
	widgetLastFarmingTime.Title = "Last farming time"
	ui.Render(widgetLastFarmingTime)

	widgetTotalFarmingPlotsNumber = widgets.NewParagraph()
	widgetTotalFarmingPlotsNumber.SetRect(smallWidgetWidth*3, 0, (smallWidgetWidth*4)-1, smallWidgetHeight)
	widgetTotalFarmingPlotsNumber.Title = "Farming attempts"
	ui.Render(widgetTotalFarmingPlotsNumber)

	widgetMinFarmingTime = widgets.NewParagraph()
	widgetMinFarmingTime.SetRect(smallWidgetWidth*4, 0, (smallWidgetWidth*5)-1, smallWidgetHeight)
	widgetMinFarmingTime.Title = "Min farming time"
	ui.Render(widgetMinFarmingTime)

	widgetMaxFarmingTime = widgets.NewParagraph()
	widgetMaxFarmingTime.SetRect(smallWidgetWidth*5, 0, (smallWidgetWidth*6)-1, smallWidgetHeight)
	widgetMaxFarmingTime.Title = "Max farming time"
	ui.Render(widgetMaxFarmingTime)

	widgetLog = widgets.NewParagraph()
	widgetLog.SetRect(0, 10, 179, smallWidgetHeight)
	widgetLog.Title = "Last farming"
	ui.Render(widgetLog)

	widgetBarChart = widgets.NewBarChart()
	widgetBarChart.Data = []float64{3, 2, 5, 3, 9, 3}
	widgetBarChart.Title = "Plots eligible for farming - last 29 values"
	widgetBarChart.SetRect(0, 10, 119, 25)
	widgetBarChart.BarColors = []ui.Color{ui.ColorGreen}
	widgetBarChart.LabelStyles = []ui.Style{ui.NewStyle(ui.ColorBlue)}
	widgetBarChart.NumStyles = []ui.Style{ui.NewStyle(ui.ColorWhite)}

	widgetBarChart2 = widgets.NewPlot()
	widgetBarChart2.Title = "Farming times (axis Y in seconds) - last 110 values"
	widgetBarChart2.Data = make([][]float64, 1)
	widgetBarChart2.SetRect(0, 25, 119, 40)
	widgetBarChart2.AxesColor = ui.ColorWhite
	widgetBarChart2.LineColors[0] = ui.ColorRed
	widgetBarChart2.Marker = widgets.MarkerBraille

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

func loopReadFile() {
	renderLog(fmt.Sprintf("Reading log %s, please wait", *debuglogFile))
	readFullFile(*debuglogFile)
	c := time.Tick(5 * time.Second)
	var actualLogFileSize int64
	for range c {
		actualLogFileSize, _ = getFileSize(*debuglogFile)
		if actualLogFileSize < lastLogFileSize { // new file ?
			readFullFile(*debuglogFile)
		} else {
			readFile(*debuglogFile)
		}

		setLastLogFileSize(*debuglogFile)
	}
}

func setLastLogFileSize(filepath string) {
	lastLogFileSize, _ = getFileSize(filepath)
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

	//setLastLogFileSize(fname)

	// os.Open() opens specific file in
	// read-only mode and this return
	// a pointer of type os.
	file, err := os.Open(fname)

	if err != nil {
		log.Fatalf("failed to open")

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
		return
	}

	//setLastLogFileSize(fname)

	file, err := os.Open(fname)
	if err != nil {
		panic(err)
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

	_, err = file.ReadAt(buf, start)
	if err == nil {
		lines := strings.Split(string(buf), "\n")
		parseLines(lines)
	}

	file.Close()
}

func parseLines(lines []string) {
	//0 plots were eligible for farming 27274481c3... Found 0 proofs. Time: 0.14447 s. Total 105 plots
	regexPlotsFarming, _ := regexp.Compile("([0-9])\\s+plots\\s+were\\seligible.*Found\\s([0-9])+\\sproofs.*Time:\\s([0-9]+\\.[0-9]+)\\ss\\.\\sTotal\\s([0-9]+)\\splots")

	startParsingLines := false
	for i, s := range lines {
		if i == 0 { //skip the first row - can be uncomplete due reading by bytes
			continue
		}
		if s == "" {
			continue
		}
		if startParsingLines == false {
			if lastRow == "" { //first run ?
				startParsingLines = true
			} else {
				if lastRow == s {
					startParsingLines = true
					continue
				} else {
					continue
				}
			}
		}

		lastRow = s

		if regexPlotsFarming.MatchString(s) == true {
			lastParsedLinesStack.push(s)

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

			parsedTime, _ := strconv.ParseFloat(farmingTime, 8)
			if parsedTime < float64(minFarmingTime) {
				minFarmingTime = parsedTime
			}
			if parsedTime > float64(maxFarmingTime) {
				maxFarmingTime = parsedTime
			}
			lastFarmingTimesStack.push(parsedTime)

			renderWidgets()
		}
	}
	renderWidgets()
	renderLastFarmBarChart()
	renderLastFarmBarChart2()

	var tmpTxt strings.Builder
	for i := range lastParsedLinesStack.lines {
		tmpTxt.WriteString(lastParsedLinesStack.lines[i])
		tmpTxt.WriteString("\n")
	}
	renderLog(tmpTxt.String())
}

func renderWidgets() {
	renderTotalFarmingPlotsNumber()
	renderLastPlots()
	renderFoundProofs()
	renderLastFarmingTime()
	renderMinFarmingTime()
	renderMaxFarmingTime()
}

func renderTotalFarmingPlotsNumber() {
	percent := 0.0
	if totalFarmingAttempt > 0 {
		percent = float64(float64(positiveFarmingAttempt)/float64(totalFarmingAttempt)) * 100
	}
	widgetTotalFarmingPlotsNumber.Text = fmt.Sprintf("%d/%d(%.2f%%)", positiveFarmingAttempt, totalFarmingAttempt, percent)
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
	widgetLastFarmingTime.Text = fmt.Sprintf("%ss", farmingTime)
	ui.Render(widgetLastFarmingTime)
}

func renderLog(text string) {
	widgetLog.Text = fmt.Sprintf("%s", text)
	ui.Render(widgetLog)
}

func renderMinFarmingTime() {
	widgetMinFarmingTime.Text = fmt.Sprintf("%fs", minFarmingTime)
	ui.Render(widgetMinFarmingTime)
}

func renderMaxFarmingTime() {
	widgetMaxFarmingTime.Text = fmt.Sprintf("%fs", maxFarmingTime)
	ui.Render(widgetMaxFarmingTime)
}

func renderLastFarmBarChart() {
	widgetBarChart.Data = lastFarmStack.values
	ui.Render(widgetBarChart)
}

func renderLastFarmBarChart2() {
	widgetBarChart2.Data[0] = lastFarmingTimesStack.values
	ui.Render(widgetBarChart2)
}
