package test

import (
	"fmt"
	"testing"

	gt "github.com/mangenotwork/gathertool"
)

const testJSON = `{
    "store": {
      "book": [
        { "category": "fiction", "author": "Author A", "title": "Book 1" },
        { "category": "reference", "author": "Author B", "title": "Book 2" },
        { "category": "fiction", "author": "Author C", "title": "Book 3" }
      ]
    }
}`

func errorsEqual(a, b error) bool {
	if a == nil && b == nil {
		return true
	}
	if a != nil && b != nil {
		return a.Error() == b.Error()
	}
	return false
}

func TestJSONPath(t *testing.T) {
	tests := []struct {
		name     string // 测试用例的别名（避免空格被替换为下划线）
		path     string // 真实的 JSONPath 表达式
		expected string
		err      error // 错误
	}{
		{
			name:     "FirstBookTitle",
			path:     "$.store.book[0].title",
			expected: `["Book 1"]`,
			err:      nil,
		},
		{
			name:     "AllAuthors",
			path:     "$.store.book[*].author",
			expected: `["Author A","Author B","Author C"]`,
			err:      nil,
		},
		{
			name:     "FictionBooks",
			path:     "$.store.book[?(@.category == 'fiction')].title", // 这里保留真实的空格
			expected: `["Book 1","Book 3"]`,
			err:      nil,
		},
		{
			name:     "RecursiveAuthors",
			path:     "$..author",
			expected: `["Author A","Author B","Author C"]`,
			err:      nil,
		},
		{
			name:     "Found",
			path:     "$.store.name",
			expected: ``,
			err:      fmt.Errorf("no results found for path '%s'", "$.store.name"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := gt.QueryJSON(testJSON, tt.path)
			t.Log("result : ", result)
			if !errorsEqual(err, tt.err) {
				t.Fatalf("Expected error '%v', got '%v'", tt.err, err)
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}

}
