// Package report defines reusable reporting abstractions for miroir workflows.
package report

// Reporter receives structured progress and error updates from miroir operations.
type Reporter interface {
	Repo(slot int, msg string)
	Remote(slot, j int, msg string)
	Output(slot, j int, msg string)
	Error(slot int, msg string)
	ErrorRemote(slot, j int, msg string)
	ErrorOutput(slot, j int, msg string)
	Clear(slot int)
	Finish()
}
