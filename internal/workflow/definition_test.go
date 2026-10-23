package workflow

import (
	"reflect"
	"testing"
)

func definition() Definition {
	return Definition{Input: "主题", Nodes: []Node{
		{Executor: "eino", ID: "summary", Model: "test", Instruction: "总结", DependsOn: []string{"left", "right"}},
		{Executor: "eino", ID: "left", Model: "test", Instruction: "分析"},
		{Executor: "eino", ID: "right", Model: "test", Instruction: "质疑"},
	}}
}

func TestLayers(t *testing.T) {
	d := definition()
	layers, err := d.Layers()
	if err != nil {
		t.Fatal(err)
	}
	var ids [][]string
	for _, layer := range layers {
		var row []string
		for _, node := range layer {
			row = append(row, node.ID)
		}
		ids = append(ids, row)
	}
	if !reflect.DeepEqual(ids, [][]string{{"left", "right"}, {"summary"}}) {
		t.Fatalf("unexpected layers: %v", ids)
	}
}

func TestInvalidDefinition(t *testing.T) {
	cases := map[string]func(*Definition){
		"empty":                func(d *Definition) { d.Nodes = nil },
		"duplicate":            func(d *Definition) { d.Nodes[2].ID = "left" },
		"unknown":              func(d *Definition) { d.Nodes[0].DependsOn = []string{"missing"} },
		"cycle":                func(d *Definition) { d.Nodes[1].DependsOn = []string{"summary"} },
		"self":                 func(d *Definition) { d.Nodes[1].DependsOn = []string{"left"} },
		"duplicate dependency": func(d *Definition) { d.Nodes[0].DependsOn = []string{"left", "left"} },
		"invalid id":           func(d *Definition) { d.Nodes[0].ID = "../secret" },
		"empty model":          func(d *Definition) { d.Nodes[0].Model = " " },
		"empty instruction":    func(d *Definition) { d.Nodes[0].Instruction = " " },
		"too many":             func(d *Definition) { d.Nodes = make([]Node, 33) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d := definition()
			mutate(&d)
			if _, err := d.Layers(); err == nil {
				t.Fatal("accepted invalid definition")
			}
		})
	}
}

func TestNodeInputContainsOnlyDirectDependencies(t *testing.T) {
	d := definition()
	got := d.NodeInput(d.Nodes[0], map[string]NodeOutput{
		"left": {Text: "L"}, "right": {Text: "R"}, "unrelated": {Text: "secret"},
	})
	if got.Input != d.Input || len(got.Parents) != 2 || got.Parents["left"].Text != "L" {
		t.Fatalf("wrong node input: %+v", got)
	}
	if _, ok := got.Parents["unrelated"]; ok {
		t.Fatal("leaked unrelated output")
	}
}
