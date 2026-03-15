package pharma

import "testing"

// AC-008: Class type parsed from FDA bracket suffix
func TestParsePharmClass_EPC(t *testing.T) {
	pc := ParsePharmClass("HMG-CoA Reductase Inhibitor [EPC]")
	if pc.Name != "HMG-CoA Reductase Inhibitor" {
		t.Errorf("Name = %q, want %q", pc.Name, "HMG-CoA Reductase Inhibitor")
	}
	if pc.Type != "EPC" {
		t.Errorf("Type = %q, want %q", pc.Type, "EPC")
	}
}

func TestParsePharmClass_MoA(t *testing.T) {
	pc := ParsePharmClass("Hydroxymethylglutaryl-CoA Reductase Inhibitors [MoA]")
	if pc.Name != "Hydroxymethylglutaryl-CoA Reductase Inhibitors" {
		t.Errorf("Name = %q, want %q", pc.Name, "Hydroxymethylglutaryl-CoA Reductase Inhibitors")
	}
	if pc.Type != "MoA" {
		t.Errorf("Type = %q, want %q", pc.Type, "MoA")
	}
}

func TestParsePharmClass_PE(t *testing.T) {
	pc := ParsePharmClass("Decreased Hepatic Cholesterol Synthesis [PE]")
	if pc.Type != "PE" {
		t.Errorf("Type = %q, want %q", pc.Type, "PE")
	}
}

func TestParsePharmClass_CS(t *testing.T) {
	pc := ParsePharmClass("Statins [CS]")
	if pc.Name != "Statins" {
		t.Errorf("Name = %q, want %q", pc.Name, "Statins")
	}
	if pc.Type != "CS" {
		t.Errorf("Type = %q, want %q", pc.Type, "CS")
	}
}

// Edge: no bracket — full string is name, type is empty
func TestParsePharmClass_NoBracket(t *testing.T) {
	pc := ParsePharmClass("Some Unknown Class")
	if pc.Name != "Some Unknown Class" {
		t.Errorf("Name = %q, want %q", pc.Name, "Some Unknown Class")
	}
	if pc.Type != "" {
		t.Errorf("Type = %q, want empty", pc.Type)
	}
}

// Edge: unknown bracket type
func TestParsePharmClass_UnknownBracket(t *testing.T) {
	pc := ParsePharmClass("Something [XYZ]")
	if pc.Name != "Something" {
		t.Errorf("Name = %q, want %q", pc.Name, "Something")
	}
	if pc.Type != "XYZ" {
		t.Errorf("Type = %q, want %q", pc.Type, "XYZ")
	}
}

func TestParsePharmClasses_Multiple(t *testing.T) {
	input := []string{
		"HMG-CoA Reductase Inhibitor [EPC]",
		"Hydroxymethylglutaryl-CoA Reductase Inhibitors [MoA]",
	}
	classes := ParsePharmClasses(input)
	if len(classes) != 2 {
		t.Fatalf("len = %d, want 2", len(classes))
	}
	if classes[0].Type != "EPC" {
		t.Errorf("classes[0].Type = %q, want EPC", classes[0].Type)
	}
	if classes[1].Type != "MoA" {
		t.Errorf("classes[1].Type = %q, want MoA", classes[1].Type)
	}
}

func TestParsePharmClasses_Nil(t *testing.T) {
	classes := ParsePharmClasses(nil)
	if classes == nil {
		t.Error("should return empty slice, not nil")
	}
	if len(classes) != 0 {
		t.Errorf("len = %d, want 0", len(classes))
	}
}
