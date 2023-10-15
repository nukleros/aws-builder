package util

// CreateMapTags creates tags in map[string]string format for AWS services that
// use that format.
func CreateMapTags(name string, tags map[string]string) map[string]string {
	var outputTags map[string]string
	if tags == nil {
		outputTags = make(map[string]string)
	} else {
		outputTags = tags
	}
	outputTags["Name"] = name
	return outputTags
}
