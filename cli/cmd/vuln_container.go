//
// Author:: Salim Afiune Maya (<afiune@lacework.net>)
// Copyright:: Copyright 2020, Lacework Inc.
// License:: Apache License, Version 2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package cmd

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/lacework/go-sdk/api"
)

var (
	// vulContainerScanCmd represents the scan sub-command inside the container vulnerability command
	vulContainerScanCmd = &cobra.Command{
		Use:   "scan <registry> <repository> <tag|digest>",
		Short: "request an on-demand container vulnerability assessment",
		Long: `Request on-demand container vulnerability assessments and view the generated results.

NOTE: Scans can take up to 15 minutes to return results.

Arguments:
  <registry>    container registry where the container image has been published
  <repository>  repository name that contains the container image
  <tag|digest>  either a tag or an image digest to scan (digest format: sha256:1ee...1d3b)`,
		Args: cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			return requestOnDemandContainerVulnerabilityScan(args)
		},
	}

	// vulContainerScanStatusCmd represents the scan-status sub-command inside the container
	// vulnerability command
	vulContainerScanStatusCmd = &cobra.Command{
		Use:     "scan-status <request_id>",
		Aliases: []string{"status"},
		Short:   "check the status of an on-demand container vulnerability assessment",
		Long:    "Check the status of an on-demand container vulnerability assessment.",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return checkOnDemandContainerVulnerabilityStatus(args[0])
		},
	}

	// vulContainerListAssessmentsCmd represents the list-assessments sub-command inside the container
	// vulnerability command
	vulContainerListAssessmentsCmd = &cobra.Command{
		Use:     "list-assessments",
		Aliases: []string{"list", "ls"},
		Short:   "list container vulnerability assessments (default last 7 days)",
		Long: `List all container vulnerability assessments for the last 7 days by default, or
pass --start and --end to specify a custom time period. You can also pass --active
to filter on active containers in your environment, as well as pass --fixable to
filter on containers with vulnerabilities that have fixes available.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			var (
				response api.VulnContainerAssessmentsResponse
				err      error
			)
			if vulCmdState.Start != "" || vulCmdState.End != "" {
				start, end, errT := parseStartAndEndTime(vulCmdState.Start, vulCmdState.End)
				if errT != nil {
					return errors.Wrap(errT, "unable to parse time range")
				}

				cli.Log.Infow("requesting list of assessments from custom time range",
					"start_time", start, "end_time", end,
				)
				response, err = cli.LwApi.Vulnerabilities.Container.ListAssessmentsDateRange(start, end)
			} else {
				cli.Log.Info("requesting list of assessments from the last 7 days")
				response, err = cli.LwApi.Vulnerabilities.Container.ListAssessments()
			}

			if err != nil {
				return errors.Wrap(err, "unable to get assessments")
			}

			cli.Log.Debugw("assessments", "raw", response)

			if len(response.Assessments) == 0 {
				cli.OutputHuman("There are no assessments of container in your environment.\n")
				return nil
			}

			// if the user wants to show only assessments of running containers
			// order them by that field, number of running containers
			if vulCmdState.Active {
				// Sort the assessments from the response by running containers
				sort.Slice(response.Assessments, func(i, j int) bool {
					return stringToInt(response.Assessments[i].NdvContainers) > stringToInt(response.Assessments[j].NdvContainers)
				})
			} else {
				// Sort the assessments from the response by date
				sort.Slice(response.Assessments, func(i, j int) bool {
					return response.Assessments[i].StartTime.ToTime().After(response.Assessments[j].StartTime.ToTime())
				})
			}

			if cli.JSONOutput() {
				return cli.OutputJSON(response.Assessments)
			}

			cli.OutputHuman(vulAssessmentsToTableReport(response.Assessments))
			return nil
		},
	}

	// vulContainerShowAssessmentCmd represents the show-assessment sub-command inside the container
	// vulnerability command
	vulContainerShowAssessmentCmd = &cobra.Command{
		Use:     "show-assessment <sha256:hash>",
		Aliases: []string{"show"},
		Short:   "show results of a container vulnerability assessment",
		Long: `Review the results from a vulnerability assessment of a container image.

Arguments:
  <sha256:hash> a sha256 hash of a container image (format: sha256:1ee...1d3b)

By default, this command treads the provided sha256 as image digest, when trying to
lookup an assessment by its image id, provided the flag '--image_id'.

To request an on-demand vulnerability scan:

    $ lacework vulnerability container scan <registry> <repository> <tag|digest>
`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return showContainerAssessmentsWithSha256(args[0])
		},
	}
)

func init() {
	// add sub-commands to the 'vulnerability container' command
	vulContainerCmd.AddCommand(vulContainerScanCmd)
	vulContainerCmd.AddCommand(vulContainerScanStatusCmd)
	vulContainerCmd.AddCommand(vulContainerListAssessmentsCmd)
	vulContainerCmd.AddCommand(vulContainerShowAssessmentCmd)

	// add start flag to list-assessments command
	vulContainerListAssessmentsCmd.Flags().StringVar(&vulCmdState.Start,
		"start", "", "start of the time range in UTC (format: yyyy-MM-ddTHH:mm:ssZ)",
	)
	// add end flag to list-assessments command
	vulContainerListAssessmentsCmd.Flags().StringVar(&vulCmdState.End,
		"end", "", "end of the time range in UTC (format: yyyy-MM-ddTHH:mm:ssZ)",
	)
	// add active flag to list-assessments command
	vulContainerListAssessmentsCmd.Flags().BoolVar(&vulCmdState.Active,
		"active", false, "only show assessments of containers actively running with vulnerabilities in your environment",
	)

	setPollFlag(
		vulContainerScanCmd.Flags(),
		vulContainerScanStatusCmd.Flags(),
	)

	setDetailsFlag(
		vulContainerScanCmd.Flags(),
		vulContainerScanStatusCmd.Flags(),
		vulContainerShowAssessmentCmd.Flags(),
	)

	setFixableFlag(
		vulContainerScanCmd.Flags(),
		vulContainerScanStatusCmd.Flags(),
		vulContainerShowAssessmentCmd.Flags(),
		vulContainerListAssessmentsCmd.Flags(),
	)

	setPackagesFlag(
		vulContainerScanCmd.Flags(),
		vulContainerScanStatusCmd.Flags(),
		vulContainerShowAssessmentCmd.Flags(),
	)

	vulContainerShowAssessmentCmd.Flags().BoolVar(
		&vulCmdState.ImageID, "image_id", false,
		"tread the provided sha256 hash as image id",
	)
}

func requestOnDemandContainerVulnerabilityScan(args []string) error {
	cli.Log.Debugw("requesting vulnerability scan",
		"registry", args[0],
		"repository", args[1],
		"tag_or_digest", args[2],
	)
	scan, err := cli.LwApi.Vulnerabilities.Container.Scan(args[0], args[1], args[2])
	if err != nil {
		return errors.Wrap(err, "unable to request on-demand vulnerability scan")
	}

	cli.Log.Debugw("vulnerability scan", "details", scan)
	if !scan.Ok {
		return errors.Errorf(
			"there is a problem with the vulnerability scan: %s",
			scan.Message,
		)
	}

	cli.OutputHuman(
		"A new vulnerability scan has been requested. (request_id: %s)\n\n",
		scan.Data.RequestID,
	)

	if vulCmdState.Poll {
		cli.Log.Infow("tracking scan progress",
			"param", "--poll",
			"request_id", scan.Data.RequestID,
		)
		return pollScanStatus(scan.Data.RequestID)
	}

	if cli.JSONOutput() {
		return cli.OutputJSON(scan.Data)
	}

	cli.OutputHuman("To track the progress of the scan, use the command:\n")
	cli.OutputHuman("  $ lacework vulnerability container scan-status %s\n", scan.Data.RequestID)
	return nil
}

func checkOnDemandContainerVulnerabilityStatus(reqID string) error {
	if vulCmdState.Poll {
		cli.Log.Infow("tracking scan progress",
			"param", "--poll",
			"request_id", reqID,
		)
		return pollScanStatus(reqID)
	}

	results, err, scanning := checkScanStatus(reqID)
	if err != nil {
		return err
	}

	if cli.JSONOutput() {
		return cli.OutputJSON(results)
	}

	// if the scan is still running, display a nice message
	if scanning {
		cli.OutputHuman(
			"The vulnerability scan is still running. (request_id: %s)\n\n",
			reqID,
		)
		cli.OutputHuman("Use '--poll' to poll until the vulnerability scan completes.\n")
		return nil
	}

	cli.OutputHuman(buildVulnerabilityReport(results))
	return nil
}

func showContainerAssessmentsWithSha256(sha string) error {
	var (
		assessment  api.VulnContainerAssessmentResponse
		searchField string
		err         error
	)
	if vulCmdState.ImageID {
		searchField = "image_id"
		cli.Log.Debugw("retrieve image assessment", searchField, sha)
		assessment, err = cli.LwApi.Vulnerabilities.Container.AssessmentFromImageID(sha)
	} else {
		searchField = "digest"
		cli.Log.Debugw("retrieve image assessment", searchField, sha)
		assessment, err = cli.LwApi.Vulnerabilities.Container.AssessmentFromImageDigest(sha)
	}
	if err != nil {
		return errors.Wrap(err, "unable to show vulnerability assessment")
	}

	cli.Log.Debugw("image assessment", "details", assessment)
	status := assessment.CheckStatus()
	switch status {
	case "Success":
		if cli.JSONOutput() {
			return cli.OutputJSON(assessment.Data)
		}

		cli.OutputHuman(buildVulnerabilityReport(&assessment.Data))
	case "Unsupported":
		return errors.Errorf(
			`unable to retrieve assessment for the provided container image. (unsupported distribution)

For more information about supported distributions, visit:
    https://support.lacework.com/hc/en-us/articles/360035472393-Container-Vulnerability-Assessment-Overview
`,
		)
	case "NotFound":
		msg := fmt.Sprintf(
			"unable to find any assessment from a container image with %s '%s'",
			searchField, sha,
		)

		// add a suggestion to the user in regards of the image_id vs digest
		if !vulCmdState.ImageID {
			msg = fmt.Sprintf("%s\n\n(?) Are you trying to lookup a container vulnerability assessment using an image id?", msg)
			msg = fmt.Sprintf("%s\n(?) Try using the flag '--image_id'", msg)
		}

		return errors.New(msg)
	case "Failed":
		return errors.New(
			"the assessment failed to execute. Use '--debug' to troubleshoot.",
		)
	default:
		return errors.New(
			"unable to get assessment status from the container image. Use '--debug' to troubleshoot.",
		)
	}

	return nil
}

func buildVulnerabilityReport(assessment *api.VulnContainerAssessment) string {
	var (
		t                 *tablewriter.Table
		imageDetailsTable = &strings.Builder{}
		vulCountsTable    = &strings.Builder{}
		mainReport        = &strings.Builder{}
	)

	if assessment.TotalVulnerabilities == 0 {
		// @afiune this emoji's do not work on Windows
		return fmt.Sprintf("Great news! This container image has no vulnerabilities... (time for %s)\n", randomEmoji())
	}

	t = tablewriter.NewWriter(imageDetailsTable)
	t.SetBorder(false)
	t.SetColumnSeparator("")
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	t.AppendBulk(vulContainerImageToTable(assessment.Image))
	t.Render()

	t = tablewriter.NewWriter(vulCountsTable)
	t.SetBorder(false)
	t.SetColumnSeparator(" ")
	t.SetHeader([]string{
		"Severity", "Count", "Fixable",
	})
	t.AppendBulk(vulContainerAssessmentToCountsTable(assessment))
	t.Render()

	t = tablewriter.NewWriter(mainReport)
	t.SetBorder(false)
	t.SetAutoWrapText(false)
	t.SetHeader([]string{
		"Container Image Details",
		"Vulnerabilities",
	})
	t.Append([]string{
		imageDetailsTable.String(),
		vulCountsTable.String(),
	})
	t.Render()

	if vulCmdState.Details || vulCmdState.Fixable || vulCmdState.Packages {
		if vulCmdState.Packages {
			mainReport.WriteString(buildVulnerabilityPackageSummary(assessment))
			mainReport.WriteString("\n")
		} else {
			mainReport.WriteString(buildVulnerabilityReportDetails(assessment))
			mainReport.WriteString("\n")
			mainReport.WriteString("Try adding '--packages' to show a list of packages with CVE count.\n")
		}
	} else {
		mainReport.WriteString(
			"Try adding '--details' to increase details shown about the vulnerability assessment.\n",
		)
	}

	return mainReport.String()
}

func buildVulnerabilityPackageSummary(assessment *api.VulnContainerAssessment) string {
	var (
		detailsTable = &strings.Builder{}
		t            = tablewriter.NewWriter(detailsTable)
	)

	t.SetRowLine(false)
	t.SetBorder(false)
	t.SetColumnSeparator(" ")
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	t.SetHeader([]string{
		"CVE Count",
		"Severity",
		"Package",
		"Current Version",
		"Fix Version",
	})
	t.AppendBulk(vulContainerImagePackagesToTable(assessment.Image))
	t.Render()

	return detailsTable.String()
}

func buildVulnerabilityReportDetails(assessment *api.VulnContainerAssessment) string {
	var (
		detailsTable = &strings.Builder{}
		t            = tablewriter.NewWriter(detailsTable)
	)

	t.SetRowLine(true)
	t.SetBorders(tablewriter.Border{
		Left:   false,
		Right:  false,
		Top:    true,
		Bottom: true,
	})
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	t.SetHeader([]string{
		"CVE",
		"Severity",
		"Package",
		"Current Version",
		"Fix Version",
		"Introduced in Layer",
	})
	t.AppendBulk(vulContainerImageLayersToTable(assessment.Image))
	t.Render()

	return detailsTable.String()
}

func vulContainerImagePackagesToTable(image *api.VulnContainerImage) [][]string {
	if image == nil {
		return [][]string{}
	}

	out := [][]string{}
	for _, layer := range image.ImageLayers {
		for _, pkg := range layer.Packages {
			for _, vul := range pkg.Vulnerabilities {
				if vulCmdState.Fixable && vul.FixVersion == "" {
					continue
				}

				added := false
				for i := range out {
					if out[i][1] == strings.Title(vul.Severity) &&
						out[i][2] == pkg.Name &&
						out[i][3] == pkg.Version &&
						out[i][4] == vul.FixVersion {

						if count, err := strconv.Atoi(out[i][0]); err == nil {
							out[i][0] = fmt.Sprintf("%d", (count + 1))
							added = true
						}

					}
				}

				if added {
					continue
				}

				out = append(out, []string{
					"1",
					strings.Title(vul.Severity),
					pkg.Name,
					pkg.Version,
					vul.FixVersion,
				})
			}
		}
	}

	// order by severity
	sort.Slice(out, func(i, j int) bool {
		return severityOrder(out[i][1]) < severityOrder(out[j][1])
	})

	return out
}

func vulContainerImageLayersToTable(image *api.VulnContainerImage) [][]string {
	if image == nil {
		return [][]string{}
	}

	out := [][]string{}
	for _, layer := range image.ImageLayers {
		for _, pkg := range layer.Packages {
			for _, vul := range pkg.Vulnerabilities {
				if vulCmdState.Fixable && vul.FixVersion == "" {
					continue
				}
				space := regexp.MustCompile(`\s+`)
				createdBy := space.ReplaceAllString(layer.CreatedBy, " ")

				out = append(out, []string{
					vul.Name,
					strings.Title(vul.Severity),
					pkg.Name,
					pkg.Version,
					vul.FixVersion,
					createdBy,
				})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return severityOrder(out[i][1]) < severityOrder(out[j][1])
	})

	return out
}

func vulContainerAssessmentToCountsTable(assessment *api.VulnContainerAssessment) [][]string {
	return [][]string{
		[]string{"Critical", fmt.Sprint(assessment.CriticalVulnerabilities),
			fmt.Sprint(assessment.VulnFixableCount("critical"))},
		[]string{"High", fmt.Sprint(assessment.HighVulnerabilities),
			fmt.Sprint(assessment.VulnFixableCount("high"))},
		[]string{"Medium", fmt.Sprint(assessment.MediumVulnerabilities),
			fmt.Sprint(assessment.VulnFixableCount("medium"))},
		[]string{"Low", fmt.Sprint(assessment.LowVulnerabilities),
			fmt.Sprint(assessment.VulnFixableCount("low"))},
		[]string{"Info", fmt.Sprint(assessment.InfoVulnerabilities),
			fmt.Sprint(assessment.VulnFixableCount("info"))},
	}
}

func vulContainerImageToTable(image *api.VulnContainerImage) [][]string {
	if image == nil || image.ImageInfo == nil {
		return [][]string{}
	}

	info := image.ImageInfo
	return [][]string{
		[]string{"ID", info.ImageID},
		[]string{"Digest", info.ImageDigest},
		[]string{"Registry", info.Registry},
		[]string{"Repository", info.Repository},
		[]string{"Size", byteCountBinary(info.Size)},
		[]string{"Created At", info.CreatedTime},
		[]string{"Tags", strings.Join(info.Tags, ",")},
	}
}

func vulAssessmentsToTableReport(assessments []api.VulnContainerAssessmentSummary) string {
	var (
		assessmentsTable = &strings.Builder{}
		t                = tablewriter.NewWriter(assessmentsTable)
		rows             = vulAssessmentsToTable(assessments)
	)

	// if the user wants to show only assessments of containers running
	// and we don't have any, show a friendly message
	if vulCmdState.Active && len(rows) == 0 {
		//@afiune add fixable flag
		return "There are no active containers in your environment.\n"
	}

	t.SetHeader([]string{
		"Registry",
		"Repository",
		"Last Scan",
		"Status",
		"Containers",
		"Vulnerabilities",
		"Image Digest",
	})
	t.SetAutoWrapText(true)
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	t.SetBorder(false)
	t.AppendBulk(rows)
	t.Render()

	if !vulCmdState.Active {
		assessmentsTable.WriteString(
			"\nTry adding '--active' to only show assessments of containers actively running with vulnerabilities.\n",
		)
	} else if !vulCmdState.Fixable {
		assessmentsTable.WriteString(
			"\nTry adding '--fixable' to only show assessments with fixable vulnerabilities.\n",
		)
	}
	return assessmentsTable.String()
}

func vulAssessmentsToTable(assessments []api.VulnContainerAssessmentSummary) [][]string {
	out := [][]string{}
	for _, assessment := range assessments {
		// do not add assessments that doesn't have running containers
		// if the user wants to show only assessments of containers running
		if vulCmdState.Active && assessment.NdvContainers == "0" {
			continue
		}
		if vulCmdState.Fixable && assessment.NumFixes == "0" {
			continue
		}

		// if an assessment is unsupported, the summary should not be generated
		var (
			assessmentSummary  = "-"
			hasVulnerabilities bool
		)
		if assessment.ImageScanStatus != "Unsupported" {
			assessmentSummary, hasVulnerabilities = vulSummaryFromAssessment(&assessment)
			if vulCmdState.Active && !hasVulnerabilities {
				continue
			}
		}

		if vulCmdState.Active && assessment.ImageScanStatus == "Unsupported" {
			continue
		}

		out = append(out, []string{
			assessment.ImageRegistry,
			assessment.ImageRepo,
			assessment.StartTime.UTC().Format(time.RFC3339),
			assessment.ImageScanStatus,
			assessment.NdvContainers,
			assessmentSummary,
			assessment.ImageDigest,
		})
	}
	return out
}

func vulSummaryFromAssessment(assessment *api.VulnContainerAssessmentSummary) (string, bool) {
	summary := []string{}

	summary = addToAssessmentSummary(summary, assessment.NumVulnerabilitiesSeverity1, "Critical")
	summary = addToAssessmentSummary(summary, assessment.NumVulnerabilitiesSeverity2, "High")
	summary = addToAssessmentSummary(summary, assessment.NumVulnerabilitiesSeverity3, "Medium")
	summary = addToAssessmentSummary(summary, assessment.NumVulnerabilitiesSeverity4, "Low")
	summary = addToAssessmentSummary(summary, assessment.NumVulnerabilitiesSeverity5, "Info")

	if len(summary) == 0 {
		return fmt.Sprintf("None! Time for %s", randomEmoji()), false
	}

	if assessment.NumFixes != "" {
		summary = append(summary, fmt.Sprintf("%s Fixable", assessment.NumFixes))
	}

	return strings.Join(summary, " "), true
}

func addToAssessmentSummary(text []string, num, severity string) []string {
	if len(text) == 0 {
		if num != "" && num != "0" {
			return append(text, fmt.Sprintf("%s %s", num, severity))
		}
	}
	return text
}
