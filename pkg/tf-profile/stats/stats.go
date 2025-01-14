package tfprofile

import (
	"bufio"
	"fmt"
	"sort"

	. "github.com/QuintenBruynseraede/tf-profile/pkg/tf-profile/aggregate"
	. "github.com/QuintenBruynseraede/tf-profile/pkg/tf-profile/core"
	. "github.com/QuintenBruynseraede/tf-profile/pkg/tf-profile/parser"
	. "github.com/QuintenBruynseraede/tf-profile/pkg/tf-profile/readers"
	. "github.com/QuintenBruynseraede/tf-profile/pkg/tf-profile/utils"
	"github.com/fatih/color"
	"github.com/rodaine/table"
)

type Stat struct {
	name  string
	value string
}

func Stats(args []string, tee bool) error {
	var file *bufio.Scanner
	var err error

	if len(args) == 1 {
		file, err = FileReader{File: args[0]}.Read()
	} else {
		file, err = StdinReader{}.Read()
	}

	if err != nil {
		return err
	}

	tflog, err := Parse(file, tee)
	if err != nil {
		return err
	}

	tflog, err = Aggregate(tflog)
	if err != nil {
		return err
	}

	err = PrintStats(tflog)
	if err != nil {
		return err
	}

	return nil
}

// Print various high-level stats about a ParsedLog
func PrintStats(log ParsedLog) error {
	headerFmt := color.New(color.FgHiBlue, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgBlue).SprintfFunc()

	tbl := table.New("Key", "Value")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	addRows(&tbl, getBasicStats(log))
	addRows(&tbl, getTimeStats(log))
	addRows(&tbl, getOperationStats(log))
	addRows(&tbl, getAfterStatusStats(log))
	addRows(&tbl, getDesiredStateStats(log))
	addRows(&tbl, getModuleStats(log))

	fmt.Println() // Create space above the table
	tbl.Print()

	return nil
}

// Helper to add multiple rows at once
func addRows(tbl *table.Table, rows []Stat) {
	for _, stat := range rows {
		(*tbl).AddRow(stat.name, stat.value)
	}
	(*tbl).AddRow("", "") // Add some spacing between sections
}

func getBasicStats(log ParsedLog) []Stat {
	NumCalls := 0
	for _, resource := range log.Resources {
		NumCalls += resource.NumCalls
	}
	return []Stat{
		{"Number of resources in configuration", fmt.Sprint(NumCalls)},
	}
}

func getTimeStats(log ParsedLog) []Stat {
	TotalTime := 0
	HighestTime := -1
	HighestResource := ""

	for name, metric := range log.Resources {
		TotalTime += int(metric.TotalTime / 1000)
		if int(metric.TotalTime) > HighestTime {
			HighestTime = int(metric.TotalTime)
			HighestResource = name
		}
	}
	return []Stat{
		{"Cumulative duration", FormatDuration(TotalTime)},
		{"Longest apply time", FormatDuration(HighestTime / 1000)},
		{"Longest apply resource", HighestResource},
	}
}

func getAfterStatusStats(log ParsedLog) []Stat {
	StatusCount := make(map[string]int)
	for _, metrics := range log.Resources {
		StatusCount[metrics.AfterStatus.String()] += metrics.NumCalls
	}

	result := []Stat{}
	for status, count := range StatusCount {
		StatName := fmt.Sprintf("Resources in state %v", status)
		result = append(result, Stat{StatName, fmt.Sprint(count)})
	}

	// Sort on name to make it consistent
	sort.Slice(result, func(i int, j int) bool {
		return result[i].name < result[j].name
	})
	return result
}

func getDesiredStateStats(log ParsedLog) []Stat {
	inDesiredState := 0
	notInDesiredState := 0

	for _, metric := range log.Resources {
		if metric.AfterStatus == metric.DesiredStatus {
			inDesiredState += 1
		} else {
			notInDesiredState += 1
		}
	}
	sum := inDesiredState + notInDesiredState

	percInDesired := 100 * float64(inDesiredState) / float64(sum)
	percNotInDesired := 100 * float64(notInDesiredState) / float64(sum)

	return []Stat{
		{"Resources in desired state", fmt.Sprintf("%v out of %v (%.1f%%)", inDesiredState, sum, percInDesired)},
		{"Resources not in desired state", fmt.Sprintf("%v out of %v (%.1f%%)", notInDesiredState, sum, percNotInDesired)},
	}
}

func getOperationStats(log ParsedLog) []Stat {

	Operations := make(map[string]int)
	for _, metrics := range log.Resources {
		Operations[metrics.Operation.String()] += metrics.NumCalls
	}

	result := []Stat{}
	for op, count := range Operations {
		StatName := fmt.Sprintf("Resources marked for operation %v", op)
		result = append(result, Stat{StatName, fmt.Sprint(count)})
	}
	return result
}

func getModuleStats(log ParsedLog) []Stat {
	LargestTopLevelModule := "/"
	LargestTopLevelModuleSize := 0
	DeepestModuleDepth := 0
	DeepestModuleName := "/"
	LargestLeafModuleSize := 0
	LargestLeafModuleName := "/"

	toplevel := make(map[string]int)
	LeafModuleCounts := make(map[string]int)

	for name, metrics := range log.Resources {
		toplevelmodule := getTopLevelModule(name)
		leafmodule := getLeafModuleName(name)

		// If created in a module and we haven't seen it
		_, seen := toplevel[toplevelmodule]
		if toplevelmodule != "" && seen == true {
			toplevel[toplevelmodule] += metrics.NumCalls
		} else if toplevelmodule != "" && seen == false {
			toplevel[toplevelmodule] = metrics.NumCalls
		}

		// New leaf module?
		_, seen = LeafModuleCounts[leafmodule]
		if leafmodule != "" && seen == true {
			LeafModuleCounts[leafmodule] += metrics.NumCalls
		} else if leafmodule != "" && seen == false {
			LeafModuleCounts[leafmodule] = metrics.NumCalls
		}

		// Is deeper submodule than seen before?
		if getModuleDepth(name) > DeepestModuleDepth {
			DeepestModuleDepth = getModuleDepth(name)
			DeepestModuleName = getModule(name)
		}
	}

	// Get largest toplevel module
	for name, count := range toplevel {
		if count > LargestTopLevelModuleSize {
			LargestTopLevelModule = name
			LargestTopLevelModuleSize = count
		}
	}

	// Get largest leaf module
	for name, count := range LeafModuleCounts {
		if count > LargestLeafModuleSize {
			LargestLeafModuleSize = count
			LargestLeafModuleName = "module." + name
		}
	}

	return []Stat{
		{"Number of top-level modules", fmt.Sprint(len(toplevel))},
		{"Largest top-level module", LargestTopLevelModule},
		{"Size of largest top-level module", fmt.Sprint(LargestTopLevelModuleSize)},
		{"Deepest module", DeepestModuleName},
		{"Deepest module depth", fmt.Sprint(DeepestModuleDepth)},
		{"Largest leaf module", LargestLeafModuleName},
		{"Size of largest leaf module", fmt.Sprint(LargestLeafModuleSize)},
	}
}
