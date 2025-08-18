package ui
import (
	"fmt"
	"github.com/fatih/color"
)
type ColorScheme struct {
	enabled       bool
	headerColor   *color.Color
	clusterColor  *color.Color
	userColor     *color.Color
	contextColor  *color.Color
	successColor  *color.Color
	errorColor    *color.Color
	warningColor  *color.Color
	infoColor     *color.Color
}
func NewColorScheme(enabled bool) *ColorScheme {
	if !enabled {
		color.NoColor = true
	}
	return &ColorScheme{
		enabled:       enabled,
		headerColor:   color.New(color.FgCyan, color.Bold),
		clusterColor:  color.New(color.FgBlue),
		userColor:     color.New(color.FgGreen),
		contextColor:  color.New(color.FgYellow),
		successColor:  color.New(color.FgGreen, color.Bold),
		errorColor:    color.New(color.FgRed, color.Bold),
		warningColor:  color.New(color.FgYellow),
		infoColor:     color.New(color.FgCyan),
	}
}
func (c *ColorScheme) Header(format string, a ...interface{}) {
	c.headerColor.Printf(format+"\n", a...)
}
func (c *ColorScheme) Cluster(text string) string {
	return c.clusterColor.Sprint(text)
}
func (c *ColorScheme) User(text string) string {
	return c.userColor.Sprint(text)
}
func (c *ColorScheme) Context(text string) string {
	return c.contextColor.Sprint(text)
}
func (c *ColorScheme) Success(format string, a ...interface{}) {
	c.successColor.Printf(format+"\n", a...)
}
func (c *ColorScheme) Error(format string, a ...interface{}) {
	c.errorColor.Printf(format+"\n", a...)
}
func (c *ColorScheme) Warning(format string, a ...interface{}) {
	c.warningColor.Printf(format+"\n", a...)
}
func (c *ColorScheme) Info(text string) string {
	return c.infoColor.Sprint(text)
}
func (c *ColorScheme) Bold(text string) string {
	return color.New(color.Bold).Sprint(text)
}
func (c *ColorScheme) CurrentMarker() string {
	return c.successColor.Sprint("*")
}
func (c *ColorScheme) FormatClusterName(name string, isCurrent bool) string {
	formatted := c.Cluster(name)
	if isCurrent {
		formatted = fmt.Sprintf("%s %s", c.CurrentMarker(), formatted)
	}
	return formatted
}
func (c *ColorScheme) FormatUserName(name string, isDuplicate bool) string {
	formatted := c.User(name)
	if isDuplicate {
		formatted = fmt.Sprintf("%s %s", formatted, c.warningColor.Sprint("(duplicate)"))
	}
	return formatted
}
func (c *ColorScheme) FormatContextName(name string, isCurrent bool) string {
	formatted := c.Context(name)
	if isCurrent {
		formatted = fmt.Sprintf("%s %s", c.CurrentMarker(), c.Bold(formatted))
	}
	return formatted
}