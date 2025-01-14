package tfprofile

import (
	"bufio"
	"fmt"

	. "github.com/QuintenBruynseraede/tf-profile/pkg/tf-profile/aggregate"
	. "github.com/QuintenBruynseraede/tf-profile/pkg/tf-profile/parser"
	. "github.com/QuintenBruynseraede/tf-profile/pkg/tf-profile/readers"
	. "github.com/QuintenBruynseraede/tf-profile/pkg/tf-profile/sort"
	. "github.com/QuintenBruynseraede/tf-profile/pkg/tf-profile/utils"

	. "github.com/QuintenBruynseraede/tf-profile/pkg/tf-profile/core"
	"github.com/fatih/color"
	"github.com/rodaine/table"
)

// Execute the `tf-profile table` command
func Table(args []string, max_depth int, tee bool, sort string) error {
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

	err = PrintTable(tflog, sort)
	if err != nil {
		return err
	}

	return nil
}

// Print a parsed log in tabular format, optionally sorting by certain columns
// sort_spec is a comma-separated list of "column_name=(asc|desc)", e.g. "n=asc,tot_time=desc"
func PrintTable(log ParsedLog, sort_spec string) error {
	headerFmt := color.New(color.FgHiBlue, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgBlue).SprintfFunc()

	tbl := table.New("resource", "n", "tot_time", "modify_started", "modify_ended", "desired_state", "operation", "final_state")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	// Sort the resources according to the sort_spec and create rows
	for _, r := range Sort(log, sort_spec) {
		for resource, metric := range log.Resources {
			if r == resource {
				tbl.AddRow(
					resource,
					(metric.NumCalls),
					FormatDuration(int(metric.TotalTime/1000)), // Display as "10s" or "1m30s"
					removeMinusOne(metric.ModificationStartedIndex),
					removeMinusOne(metric.ModificationCompletedIndex),
					(metric.DesiredStatus),
					(metric.Operation),
					(metric.AfterStatus),
				)
				break
			}
		}
	}

	fmt.Println() // Create space above the table
	tbl.Print()

	return nil
}

// Many metrics use -1 as value for "unknown at the time". When a resource change fails,
// these initial values remain in the log. Before printing, we replace then with '/'
func removeMinusOne(val int) string {
	if val == -1 {
		return "/"
	} else {
		return fmt.Sprintf("%v", val)
	}
}
