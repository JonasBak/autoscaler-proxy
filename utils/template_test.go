package utils

import "testing"

func assertEq(t *testing.T, a, b string) {
	if a != b {
		t.Errorf("Expected '%s' and '%s' to be equal", a, b)
	}
}

func TestFlatMap(t *testing.T) {
	config := map[string]string{
		"field_a": "${FIELD_A}",
		"field_b": "${FIELD_B}",
		"field_c": "${env.FIELD_C}",
		"field_d": "${FIELD_D}",
		"field_e": "FIELD_E",
		"field_f": "- ${FIELD_A} - ${FIELD_B} -",
	}
	replace := map[string]string{
		"FIELD_A":     "A",
		"FIELD_B":     "B",
		"env.FIELD_C": "C",
		"FIELD_E":     "E",
	}
	result := BuildTemplate(TemplateMap(replace), config).(map[string]string)

	assertEq(t, result["field_a"], replace["FIELD_A"])
	assertEq(t, result["field_b"], replace["FIELD_B"])
	assertEq(t, result["field_c"], replace["env.FIELD_C"])

	assertEq(t, result["field_d"], config["field_d"])
	assertEq(t, result["field_e"], config["field_e"])

	assertEq(t, result["field_f"], "- A - B -")
}

func TestNestedMap(t *testing.T) {
	config := map[string]map[string]string{
		"upper_a": {
			"lower_a": "${ABC}",
			"lower_b": "${DEF}",
		},
		"upper_b": {
			"lower_a": "${ABC}",
			"lower_b": "${DEF}",
		},
	}
	replace := map[string]string{
		"ABC": "123",
		"DEF": "456",
	}
	result := BuildTemplate(TemplateMap(replace), config).(map[string]map[string]string)

	assertEq(t, result["upper_a"]["lower_a"], replace["ABC"])
	assertEq(t, result["upper_a"]["lower_b"], replace["DEF"])
	assertEq(t, result["upper_b"]["lower_a"], replace["ABC"])
	assertEq(t, result["upper_b"]["lower_b"], replace["DEF"])
}

func TestNestedList(t *testing.T) {
	config := map[string][]string{
		"upper_a": {
			"${ABC}",
			"${DEF}",
		},
		"upper_b": {
			"${ABC}",
			"${DEF}",
		},
	}
	replace := map[string]string{
		"ABC": "123",
		"DEF": "456",
	}
	result := BuildTemplate(TemplateMap(replace), config).(map[string][]string)

	assertEq(t, result["upper_a"][0], replace["ABC"])
	assertEq(t, result["upper_a"][1], replace["DEF"])
	assertEq(t, result["upper_b"][0], replace["ABC"])
	assertEq(t, result["upper_b"][1], replace["DEF"])
}

func TestNestedListMap(t *testing.T) {
	config := map[string][]map[string]string{
		"upper_a": {
			{
				"lower_a": "${ABC}",
				"lower_b": "${DEF}",
			},
			{
				"lower_a": "${ABC}",
				"lower_b": "${DEF}",
			},
		},
		"upper_b": {
			{
				"lower_a": "${ABC}",
				"lower_b": "${DEF}",
			},
			{
				"lower_a": "${ABC}",
				"lower_b": "${DEF}",
			},
		},
	}
	replace := map[string]string{
		"ABC": "123",
		"DEF": "456",
	}
	result := BuildTemplate(TemplateMap(replace), config).(map[string][]map[string]string)

	assertEq(t, result["upper_a"][0]["lower_a"], replace["ABC"])
	assertEq(t, result["upper_a"][0]["lower_b"], replace["DEF"])
	assertEq(t, result["upper_a"][1]["lower_a"], replace["ABC"])
	assertEq(t, result["upper_a"][1]["lower_b"], replace["DEF"])
	assertEq(t, result["upper_b"][0]["lower_a"], replace["ABC"])
	assertEq(t, result["upper_b"][0]["lower_b"], replace["DEF"])
	assertEq(t, result["upper_b"][1]["lower_a"], replace["ABC"])
	assertEq(t, result["upper_b"][1]["lower_b"], replace["DEF"])
}
