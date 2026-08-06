package compositor

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

type WindowRule struct {
	Name string

	AppID     string
	Class     string
	Instance  string
	Title     string
	Dialog    bool
	HasDialog bool
	Modal     bool
	HasModal  bool

	Actions WindowRuleActions
}

type WindowRuleActions struct {
	Floating          bool
	HasFloating       bool
	Centered          bool
	HasCentered       bool
	KeepAbove         bool
	HasKeepAbove      bool
	Opacity           float64
	HasOpacity        bool
	Workspace         int
	HasWorkspace      bool
	Output            string
	HasOutput         bool
	Width             int
	HasWidth          bool
	Height            int
	HasHeight         bool
	X                 int
	HasX              bool
	Y                 int
	HasY              bool
	Fullscreen        bool
	HasFullscreen     bool
	Focus             bool
	HasFocus          bool
	Border            bool
	HasBorder         bool
	BorderSize        int
	HasBorderSize     bool
	BorderRounding    int
	HasBorderRounding bool
}

func windowRuleSectionName(section string) (string, bool) {
	const prefix = "window-rule "
	if !strings.HasPrefix(section, prefix) {
		return "", false
	}
	name := strings.TrimSpace(section[len(prefix):])
	return name, name != ""
}

func parseWindowRuleSetting(rule *WindowRule, key, value string) error {
	if rule == nil {
		return fmt.Errorf("nil rule")
	}
	key = strings.ToLower(strings.TrimSpace(key))
	value = unquoteRuleValue(value)
	actions := &rule.Actions
	switch key {
	case "app_id":
		rule.AppID = value
		return validateRulePattern(value)
	case "class":
		rule.Class = value
		return validateRulePattern(value)
	case "instance":
		rule.Instance = value
		return validateRulePattern(value)
	case "title":
		rule.Title = value
		return validateRulePattern(value)
	case "dialog":
		return parseRuleBool(value, &rule.Dialog, &rule.HasDialog)
	case "modal":
		return parseRuleBool(value, &rule.Modal, &rule.HasModal)
	case "floating":
		return parseRuleBool(value, &actions.Floating, &actions.HasFloating)
	case "centered", "centered_on_output":
		return parseRuleBool(value, &actions.Centered, &actions.HasCentered)
	case "keep_above":
		return parseRuleBool(value, &actions.KeepAbove, &actions.HasKeepAbove)
	case "fullscreen":
		return parseRuleBool(value, &actions.Fullscreen, &actions.HasFullscreen)
	case "focus":
		return parseRuleBool(value, &actions.Focus, &actions.HasFocus)
	case "border":
		return parseRuleBool(value, &actions.Border, &actions.HasBorder)
	case "opacity":
		n, err := strconv.ParseFloat(value, 64)
		if err != nil || n < 0 || n > 1 {
			return fmt.Errorf("expected a value from 0.0 to 1.0")
		}
		actions.Opacity, actions.HasOpacity = n, true
	case "workspace":
		n, err := parseRuleInt(value, 1, 99)
		if err != nil {
			return err
		}
		actions.Workspace, actions.HasWorkspace = n, true
	case "output":
		if value == "" {
			return fmt.Errorf("output name cannot be empty")
		}
		actions.Output, actions.HasOutput = value, true
	case "width":
		return parseRuleDimension(value, &actions.Width, &actions.HasWidth)
	case "height":
		return parseRuleDimension(value, &actions.Height, &actions.HasHeight)
	case "x":
		return parseRuleCoordinate(value, &actions.X, &actions.HasX)
	case "y":
		return parseRuleCoordinate(value, &actions.Y, &actions.HasY)
	case "size":
		return parseRulePair(value, 'x',
			&actions.Width, &actions.HasWidth,
			&actions.Height, &actions.HasHeight, 1, 65535)
	case "position":
		return parseRulePair(value, ',',
			&actions.X, &actions.HasX,
			&actions.Y, &actions.HasY, -65535, 65535)
	case "border_size":
		n, err := parseRuleInt(value, 0, 32)
		if err != nil {
			return err
		}
		actions.BorderSize, actions.HasBorderSize = n, true
	case "border_rounding":
		n, err := parseRuleInt(value, 0, 128)
		if err != nil {
			return err
		}
		actions.BorderRounding, actions.HasBorderRounding = n, true
	default:
		return fmt.Errorf("unknown window rule setting %q", key)
	}
	return nil
}

func unquoteRuleValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
		(value[0] == '\'' && value[len(value)-1] == '\'')) {
		return value[1 : len(value)-1]
	}
	return value
}

func validateRulePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("match pattern cannot be empty")
	}
	if _, err := path.Match(strings.ToLower(pattern), "value"); err != nil {
		return fmt.Errorf("invalid match pattern: %w", err)
	}
	return nil
}

func parseRuleBool(value string, target *bool, present *bool) error {
	b, err := strconv.ParseBool(strings.ToLower(value))
	if err != nil {
		return fmt.Errorf("expected true or false")
	}
	*target, *present = b, true
	return nil
}

func parseRuleInt(value string, min, max int) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("expected an integer from %d to %d", min, max)
	}
	return n, nil
}

func parseRuleDimension(value string, target *int, present *bool) error {
	n, err := parseRuleInt(value, 1, 65535)
	if err != nil {
		return err
	}
	*target, *present = n, true
	return nil
}

func parseRuleCoordinate(value string, target *int, present *bool) error {
	n, err := parseRuleInt(value, -65535, 65535)
	if err != nil {
		return err
	}
	*target, *present = n, true
	return nil
}

func parseRulePair(value string, separator byte,
	first *int, hasFirst *bool, second *int, hasSecond *bool,
	min, max int,
) error {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(value)), string(separator))
	if len(parts) != 2 {
		return fmt.Errorf("expected two integers separated by %q", separator)
	}
	a, err := parseRuleInt(parts[0], min, max)
	if err != nil {
		return err
	}
	b, err := parseRuleInt(parts[1], min, max)
	if err != nil {
		return err
	}
	*first, *hasFirst = a, true
	*second, *hasSecond = b, true
	return nil
}
