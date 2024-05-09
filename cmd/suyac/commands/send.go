package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

var (
	flagProjectFile string
)

func init() {
	sendCmd.PersistentFlags().StringVarP(&flagProjectFile, "project_file", "F", suyac.DefaultProjectPath, "Use the specified file for project data instead of "+suyac.DefaultProjectPath)
	sendCmd.PersistentFlags().StringArrayVarP(&flagVars, "var", "V", []string{}, "Temporarily set a variable's value for the current request only. Format is name:value")

	setupRequestOutputFlags("suyac send", sendCmd)

	rootCmd.AddCommand(sendCmd)
}

type sendOptions struct {
	projFile    string
	oneTimeVars map[string]string
	outputCtrl  suyac.OutputControl
}

var sendCmd = &cobra.Command{
	Use:     "send REQ [-F project_file]",
	Short:   "Send a request defined in a template (req)",
	Long:    "Send a request by building it from a request template (req) stored in the project.",
	Args:    cobra.ExactArgs(1),
	GroupID: "sending",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := sendFlagsToOptions()
		if err != nil {
			return err
		}

		return invokeSend(args[0], opts)
	},
}

func sendFlagsToOptions() (sendOptions, error) {
	opts := sendOptions{}

	opts.projFile = flagProjectFile
	if opts.projFile == "" {
		return opts, fmt.Errorf("project file is set to empty string")
	}

	var err error
	opts.outputCtrl, err = gatherRequestOutputFlags("suyac send")
	if err != nil {
		return opts, err
	}

	// check vars
	if len(flagVars) > 0 {
		oneTimeVars := make(map[string]string)
		for idx, v := range flagVars {
			parts := strings.SplitN(v, ":", 2)
			if len(parts) != 2 {
				return opts, fmt.Errorf("var #%d (%q) is not in format key:value", idx+1, v)
			}
			oneTimeVars[parts[0]] = parts[1]
		}
		opts.oneTimeVars = oneTimeVars
	}

	return opts, nil
}

// invokeRequest receives named vars and checked/defaulted requestOptions.
func invokeSend(reqName string, opts sendOptions) error {
	// load the project file
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqName = strings.ToLower(reqName)

	// check if the project already has a request with the same name
	tmpl, ok := p.Templates[reqName]
	if !ok {
		return fmt.Errorf("no request template %s", reqName)
	}

	if tmpl.Method == "" {
		return fmt.Errorf("request template %s has no method set", reqName)
	}

	if tmpl.URL == "" {
		return fmt.Errorf("request template %s has no URL set", reqName)
	}

	varSymbol := "$"

	sendOpts := suyac.SendOptions{
		Vars:    opts.oneTimeVars,
		Body:    tmpl.Body,
		Headers: tmpl.Headers,
		Output:  opts.outputCtrl,
	}

	capVarNames := []string{}
	for k := range tmpl.Captures {
		capVarNames = append(capVarNames, k)
	}
	sort.Strings(capVarNames)
	for _, k := range capVarNames {
		sendOpts.Captures = append(sendOpts.Captures, tmpl.Captures[k])
	}

	result, err := suyac.Send(tmpl.Method, tmpl.URL, varSymbol, sendOpts)
	if err != nil {
		return err
	}

	// TODO: persist caps

	// persist history

	if p.Config.RecordHistory {
		entry := suyac.HistoryEntry{
			Template: tmpl.Name,
			ReqTime:  result.SendTime,
			RespTime: result.RecvTime,
			Request:  result.Request,
			Response: result.Response,
			Captures: result.Captures,
		}

		p.History = append(p.History, entry)
		return p.PersistHistoryToDisk()
	}

	return nil
}
