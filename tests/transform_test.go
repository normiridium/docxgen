package tests

import (
	"docxgen"
	"testing"
)

func TestTransformTemplate(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// базовые
		{"{simple}", "{.simple}"},

		// declension с кавычками и фигурной скобкой внутри
		{`{fio|declension:` + "`genitive`:`ф: и }о`" + `}`, `{.fio | declension "genitive" "ф: и }о"}`},
		{`{fio|declension:` + "`genitive`:`ф: и |о:u`" + `}`, `{.fio | declension "genitive" "ф: и |о:u"}`},

		// truncate с числом и строковым литералом
		{`{title|truncate:10:` + "`...`" + `}`, `{.title | truncate 10 "..."}`},
		{`{title|truncate:15:` + "`...`" + `}`, `{.title | truncate 15 "..."}`},

		// числовые строки и числа
		{`{tagname|mod5:` + "`7`:`12`:8}", `{.tagname | mod5 "7" "12" 8}`},

		// prefix + default
		{`{company|prefix:` + "`ООО `" + `}`, `{.company | prefix "ООО "}`},
		{`{department|default:` + "`не указано`" + `}`, `{.department | default "не указано"}`},

		// filled
		{`{project.budget|filled:` + "`есть`" + `}`, `{.project.budget | filled "есть"}`},

		// вложенные пути
		{`{signer.fio|postfix:` + "` (подпись)`" + `}`, `{.signer.fio | postfix " (подпись)"}`},

		// необычные символы внутри литерала
		{`{text|replace:` + "`a`:`б:в}г`" + `}`, `{.text | replace "a" "б:в}г"}`},

		// готовый синтаксис (одинарные скобки) — не меняем
		{`{.fio | prefix "ООО "}`, `{.fio | prefix "ООО "}`},
		{`{.title | truncate 10 "..."}`, `{.title | truncate 10 "..."}`},
		{`{if .department}{.department}{else}нет отдела{end}`, `{if .department}{.department}{else}нет отдела{end}`},
	}

	for _, c := range cases {
		got := docxgen.TransformTemplate(c.in)
		if got != c.want {
			t.Errorf("\ninput: %s\n got : %s\nwant: %s", c.in, got, c.want)
		}
	}
}
