/**

Output package is used to output the analyze result in different formats.

*/

package output

// outputFormat is the output format of the codetalks
const OutputFormatTable string = "table"
const OutputFormatJSON string = "json"

func Output(outputFormat string) {
	switch outputFormat {
	case OutputFormatTable:
		OutputCliTable()
	case OutputFormatJSON:
		OutputJson()
	default:
		OutputCliTable()
	}
}
