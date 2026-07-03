/*
*	Description : JSONPath 是一种用于查询和过滤 JSON 数据的轻量级查询语言，其语法类似于 XPath。
*	Author 		: ManGe
*	Mail 		: 2912882908@qq.com
**/

package gathertool

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"unicode"
)

/*
JSONPath 是一种用于查询和过滤 JSON 数据的轻量级查询语言，其语法类似于 XPath。

JSONPath 语法速查：
## 1. 基本结构
- **$**: 代表 JSON 文档的根元素。
- **.**: 子节点操作符，用于访问对象的属性。例如 `$.name` 访问根对象的 `name` 属性。
- **..**: 递归下降操作符，会递归地搜索所有匹配的子节点，无论嵌套多深。例如 `$..author` 会查找所有名为 `author` 的字段。

## 2. 通配符与索引
- **\***: 通配符，匹配所有属性或元素。例如 `$.books[*].title` 获取 `books` 数组中所有书的标题。
- **[n]**: 数组索引，访问数组中指定索引的元素（从 0 开始）。例如 `$.books[0]` 获取第一本书。
- **[-n]**: 负向索引，从数组末尾开始计数。例如 `$.books[-1]` 获取最后一本书。
- **[start:end]**: 数组切片，获取从 `start` 到 `end`（不包含）的子数组。例如 `$.books[1:3]` 获取第二和第三本书。

## 3. 过滤器 (Filter)
过滤器用于根据条件筛选数组元素，语法为 `[?(@.property operator value))]`。
- **@**: 代表当前正在处理的节点。
- **支持的运算符**: `==` (等于), `!=` (不等于), `<` (小于), `<=` (小于等于), `>` (大于), `>=` (大于等于)。

**示例:**
- `$.books[?(@.price < 10)]`: 筛选出价格低于 10 的所有书籍。
- `$.books[?(@.category == 'fiction'))]`: 筛选出类别为 'fiction' 的所有书籍。

测试见 jsonpath_test.go

*/

// QueryJSON 查询json, data为原生json字符串, path为jsonPath, 输出查询的json字符串和错误
func QueryJSON(data string, path string) (string, error) {
	var jsonData interface{}
	if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
		return "", fmt.Errorf("invalid json format: %w", err)
	}
	ast, err := parse(path)
	if err != nil {
		return "", err
	}
	res, err := ast.eval(jsonData, jsonData)
	if err != nil {
		return "", err
	}
	if res == nil || len(res) == 0 {
		return "", fmt.Errorf("no results found for path '%s'", path)
	}
	b, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(b), nil
}

// AST 节点定义
type node interface {
	eval(root, current interface{}) ([]interface{}, error)
}

type pathNode struct {
	segments []node
}

type childNode struct {
	name string
}

type wildcardNode struct{}

type recursiveNode struct {
	name string // 如果为 "*"，表示递归匹配所有
}

type indexNode struct {
	index int
}

type filterNode struct {
	expr expression
}

// expression 用于过滤条件
type expression interface {
	evalBool(root, current interface{}) (bool, error)
}

type eqExpr struct {
	path  string
	value string
}

func (p *pathNode) eval(root, current interface{}) ([]interface{}, error) {
	results := []interface{}{current}
	var err error
	for _, seg := range p.segments {
		var nextResults []interface{}
		for _, res := range results {
			var vals []interface{}
			vals, err = seg.eval(root, res)
			if err != nil {
				return nil, err
			}
			nextResults = append(nextResults, vals...)
		}
		results = nextResults
	}
	return results, nil
}

func (c *childNode) eval(root, current interface{}) ([]interface{}, error) {
	v := reflect.ValueOf(current)
	if v.Kind() == reflect.Map {
		val := v.MapIndex(reflect.ValueOf(c.name))
		if val.IsValid() {
			return []interface{}{val.Interface()}, nil
		}
	}
	return []interface{}{}, nil
}

func (w *wildcardNode) eval(root, current interface{}) ([]interface{}, error) {
	v := reflect.ValueOf(current)
	var results []interface{}
	switch v.Kind() {
	case reflect.Map:
		for _, key := range v.MapKeys() {
			results = append(results, v.MapIndex(key).Interface())
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			results = append(results, v.Index(i).Interface())
		}
	}
	return results, nil
}

func (r *recursiveNode) eval(root, current interface{}) ([]interface{}, error) {
	var results []interface{}
	var walk func(interface{})
	walk = func(node interface{}) {
		v := reflect.ValueOf(node)
		switch v.Kind() {
		case reflect.Map:
			for _, key := range v.MapKeys() {
				if r.name == "*" || key.String() == r.name {
					results = append(results, v.MapIndex(key).Interface())
				}
				walk(v.MapIndex(key).Interface())
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i).Interface())
			}
		}
	}
	walk(current)
	return results, nil
}

func (i *indexNode) eval(root, current interface{}) ([]interface{}, error) {
	v := reflect.ValueOf(current)
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		idx := i.index
		if idx < 0 {
			idx = v.Len() + idx // 支持负数索引
		}
		if idx >= 0 && idx < v.Len() {
			elem := v.Index(idx).Interface()
			return []interface{}{elem}, nil
		}
	}
	return []interface{}{}, nil
}

func (f *filterNode) eval(root, current interface{}) ([]interface{}, error) {
	v := reflect.ValueOf(current)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return []interface{}{}, nil
	}
	var results []interface{}
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i).Interface()
		ok, err := f.expr.evalBool(root, item)
		if err != nil {
			return nil, err
		}
		if ok {
			results = append(results, item)
		} else {
		}
	}
	return results, nil
}

func (e *eqExpr) evalBool(root, current interface{}) (bool, error) {
	v := reflect.ValueOf(current)
	if v.Kind() == reflect.Map {
		val := v.MapIndex(reflect.ValueOf(e.path))
		if val.IsValid() {
			valKind := val.Kind()
			switch valKind {
			case reflect.String:
				return val.String() == e.value, nil
			case reflect.Float64:
				expected, err := strconv.ParseFloat(e.value, 64)
				if err != nil {
					return false, fmt.Errorf("expected value '%s' is not a number", e.value)
				}
				return val.Float() == expected, nil
			case reflect.Bool:
				expected, err := strconv.ParseBool(e.value)
				if err != nil {
					return false, fmt.Errorf("expected value '%s' is not a boolean", e.value)
				}
				return val.Bool() == expected, nil
			default:
				return fmt.Sprintf("%v", val.Interface()) == e.value, nil
			}
		}
	}
	return false, nil
}

// parser 解析器
type parser struct {
	input string
	pos   int
}

func parse(path string) (node, error) {
	p := &parser{input: path}
	return p.parsePath()
}

func (p *parser) parsePath() (node, error) {
	if p.pos >= len(p.input) || p.input[p.pos] != '$' {
		return nil, fmt.Errorf("JSONPath must start with '$'")
	}
	p.pos++ // skip '$'
	var segments []node
	for p.pos < len(p.input) {
		seg, err := p.parseSegment()
		if err != nil {
			return nil, err
		}
		if seg != nil {
			segments = append(segments, seg)
		} else {
			break
		}
	}
	return &pathNode{segments: segments}, nil
}

func (p *parser) parseSegment() (node, error) {
	if p.pos >= len(p.input) {
		return nil, nil
	}
	ch := p.input[p.pos]
	if ch == '.' {
		p.pos++
		if p.pos < len(p.input) && p.input[p.pos] == '.' {
			p.pos++
			name := p.parseIdentifier()
			return &recursiveNode{name: name}, nil
		}
		if p.pos < len(p.input) && p.input[p.pos] == '*' {
			p.pos++
			return &wildcardNode{}, nil
		}
		name := p.parseIdentifier()
		if name == "" {
			return nil, fmt.Errorf("expected identifier after '.'")
		}
		return &childNode{name: name}, nil
	}

	if ch == '[' {
		p.pos++ // skip '['
		p.skipWhitespace()

		// 处理通配符 [*]
		if p.pos < len(p.input) && p.input[p.pos] == '*' {
			p.pos++ // skip '*'
			p.skipWhitespace()
			if p.pos >= len(p.input) || p.input[p.pos] != ']' {
				return nil, fmt.Errorf("expected ']' after '*'")
			}
			p.pos++ // skip ']'
			return &wildcardNode{}, nil
		}

		// 处理过滤表达式 [?(...)]
		if p.pos < len(p.input) && p.input[p.pos] == '?' {
			p.pos++ // skip '?'
			p.skipWhitespace()
			expr, err := p.parseFilterExpr()
			if err != nil {
				return nil, err
			}
			p.skipWhitespace()
			if p.pos >= len(p.input) || p.input[p.pos] != ']' {
				return nil, fmt.Errorf("expected ']' after filter")
			}
			p.pos++ // skip ']'
			return &filterNode{expr: expr}, nil
		}

		// 处理数字索引 [n]
		numStr := ""
		for p.pos < len(p.input) && (unicode.IsDigit(rune(p.input[p.pos])) || p.input[p.pos] == '-') {
			numStr += string(p.input[p.pos])
			p.pos++
		}
		p.skipWhitespace()
		if p.pos >= len(p.input) || p.input[p.pos] != ']' {
			return nil, fmt.Errorf("expected ']' after index")
		}
		p.pos++ // skip ']'
		idx, _ := strconv.Atoi(numStr)
		return &indexNode{index: idx}, nil
	}

	return nil, nil
}

func (p *parser) parseIdentifier() string {
	start := p.pos
	for p.pos < len(p.input) && (unicode.IsLetter(rune(p.input[p.pos])) || unicode.IsDigit(rune(p.input[p.pos])) || p.input[p.pos] == '_') {
		p.pos++
	}
	return p.input[start:p.pos]
}

func (p *parser) parseFilterExpr() (expression, error) {
	if p.pos >= len(p.input) || p.input[p.pos] != '(' {
		return nil, fmt.Errorf("expected '(' in filter")
	}
	p.pos++ // skip '('
	p.skipWhitespace()

	// 解析 @.property
	if p.pos >= len(p.input) || p.input[p.pos] != '@' {
		return nil, fmt.Errorf("expected '@' in filter")
	}
	p.pos++ // skip '@'
	if p.pos >= len(p.input) || p.input[p.pos] != '.' {
		return nil, fmt.Errorf("expected '.' after '@'")
	}
	p.pos++ // skip '.'
	prop := p.parseIdentifier()

	p.skipWhitespace()
	if p.pos+1 >= len(p.input) || p.input[p.pos] != '=' || p.input[p.pos+1] != '=' {
		return nil, fmt.Errorf("expected '==' in filter")
	}
	p.pos += 2 // skip '=='
	p.skipWhitespace()

	// 解析字符串字面量 (支持单引号和双引号)
	if p.pos >= len(p.input) || (p.input[p.pos] != '\'' && p.input[p.pos] != '"') {
		return nil, fmt.Errorf("expected string literal in filter")
	}

	quote := p.input[p.pos]
	p.pos++ // skip opening quote
	start := p.pos
	for p.pos < len(p.input) && p.input[p.pos] != quote {
		p.pos++
	}
	val := p.input[start:p.pos]
	p.pos++ // skip closing quote

	p.skipWhitespace()
	if p.pos >= len(p.input) || p.input[p.pos] != ')' {
		return nil, fmt.Errorf("expected ')' in filter")
	}
	p.pos++ // skip ')'

	return &eqExpr{path: prop, value: val}, nil
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}
