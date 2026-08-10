package compositor

import "testing"

func TestWindowRuleMatchesWaylandAndDialogMetadata(t *testing.T) {
	rule := WindowRule{
		AppID: "com.example.*", Title: "*picker*",
		Dialog: true, HasDialog: true,
		Modal: true, HasModal: true,
	}
	view := &View{
		AppID: "COM.EXAMPLE.Color", Title: "Color Picker",
		Dialog: true, Modal: true,
	}
	if !rule.matches(view) {
		t.Fatal("matching Wayland dialog did not match")
	}
	view.Modal = false
	if rule.matches(view) {
		t.Fatal("non-modal window matched modal rule")
	}
}

func TestWindowRuleMatchesXWaylandClassAndInstance(t *testing.T) {
	rule := WindowRule{Class: "discord", Instance: "discord-*"}
	view := &View{XWaylandClass: "Discord", XWaylandInstance: "discord-main"}
	if !rule.matches(view) {
		t.Fatal("case-insensitive XWayland class/instance match failed")
	}
	view.XWaylandClass = "kitty"
	if rule.matches(view) {
		t.Fatal("different XWayland class matched")
	}
}

func TestResolveWindowRulesMergesInConfigurationOrder(t *testing.T) {
	server := &Server{config: Config{WindowRules: []WindowRule{
		{Name: "application", AppID: "com.example.*", Actions: WindowRuleActions{
			Floating: true, HasFloating: true,
			Opacity: 0.8, HasOpacity: true,
			Workspace: 2, HasWorkspace: true,
			UrgentOnTitleChange: true, HasUrgentOnTitleChange: true,
		}},
		{Name: "specific title", Title: "Editor", Actions: WindowRuleActions{
			Opacity: 1, HasOpacity: true,
			Border: false, HasBorder: true,
		}},
	}}}
	view := &View{AppID: "com.example.Editor", Title: "Editor"}
	actions, names := server.resolveWindowRules(view)
	if names != "application,specific title" ||
		!actions.HasFloating || !actions.Floating ||
		!actions.HasOpacity || actions.Opacity != 1 ||
		!actions.HasWorkspace || actions.Workspace != 2 ||
		!actions.HasUrgentOnTitleChange || !actions.UrgentOnTitleChange ||
		!actions.HasBorder || actions.Border {
		t.Fatalf("unexpected merged actions (%s): %+v", names, actions)
	}
}

func TestWindowRuleParsesUrgentOnTitleChange(t *testing.T) {
	rule := WindowRule{Name: "zed"}
	if err := parseWindowRuleSetting(&rule, "urgent_on_title_change", "true"); err != nil {
		t.Fatal(err)
	}
	if !rule.Actions.HasUrgentOnTitleChange || !rule.Actions.UrgentOnTitleChange {
		t.Fatalf("urgent action was not parsed: %+v", rule.Actions)
	}
}

func TestUrgentOnTitleChangeOnlyMarksHiddenMappedWindows(t *testing.T) {
	view := &View{
		Mapped: true,
		Title:  "file.txt — Zed",
		RuleActions: WindowRuleActions{
			UrgentOnTitleChange: true, HasUrgentOnTitleChange: true,
		},
	}
	if !shouldMarkUrgentOnTitleChange(view, "Zed", false) {
		t.Fatal("hidden mapped window with a changed title was not marked")
	}
	if shouldMarkUrgentOnTitleChange(view, "Zed", true) {
		t.Fatal("visible window was marked urgent")
	}
	if shouldMarkUrgentOnTitleChange(view, view.Title, false) {
		t.Fatal("unchanged title was marked urgent")
	}
	view.RuleActions.UrgentOnTitleChange = false
	if shouldMarkUrgentOnTitleChange(view, "Zed", false) {
		t.Fatal("disabled rule action marked the window urgent")
	}
}

func TestRuleGeometryImpliesFloatingUnlessExplicitlyDisabled(t *testing.T) {
	view := &View{RuleActions: WindowRuleActions{Width: 500, HasWidth: true}}
	if !view.shouldAutoFloat() {
		t.Fatal("rule dimensions did not imply floating")
	}
	view.RuleActions.Floating = false
	view.RuleActions.HasFloating = true
	if view.shouldAutoFloat() {
		t.Fatal("explicit floating=false did not override geometry implication")
	}
}

func TestOutputRuleImpliesFloatingPlacement(t *testing.T) {
	view := &View{RuleActions: WindowRuleActions{Output: "DP-2", HasOutput: true}}
	if !view.shouldAutoFloat() {
		t.Fatal("output assignment did not imply floating placement")
	}
}

func TestApplyWindowRulesRefreshesDerivedDialogFloatingState(t *testing.T) {
	server := &Server{config: Config{Tiling: true}}
	view := &View{Managed: true, Dialog: true}
	server.applyWindowRules(view, false)
	if !view.AutoFloating {
		t.Fatal("dialog metadata did not update the derived floating state")
	}

	view.Dialog = false
	server.applyWindowRules(view, false)
	if view.AutoFloating {
		t.Fatal("cleared dialog metadata left the view floating")
	}
}

func TestWindowRuleParserRejectsInvalidValues(t *testing.T) {
	for key, value := range map[string]string{
		"opacity": "1.5", "workspace": "0", "size": "800",
		"position": "10;20", "floating": "sometimes",
		"app_id": "[broken", "unknown": "value",
	} {
		rule := WindowRule{Name: "invalid"}
		if err := parseWindowRuleSetting(&rule, key, value); err == nil {
			t.Errorf("%s=%s unexpectedly accepted", key, value)
		}
	}
}
