package youtube

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
)

// YouTube's responses are order-sensitive: of several competing keys, IDs, or
// thumbnails, the first one a walk finds is the one the app uses. Decoding an
// object into a map loses that member order, so the parsers used to sort keys
// and walk them alphabetically, which both costs time and can pick a member the
// response never intended. jsonObject keeps its members in document order, and
// the parsers walk them the way the response listed them.

// jsonMember is one member of a JSON object, kept in document order.
type jsonMember struct {
	key   string
	value any
}

// jsonObject is a JSON object whose members keep their document order.
type jsonObject struct {
	members []jsonMember
}

// get returns the value stored under key, or nil when the object has none.
func (o *jsonObject) get(key string) any {
	if o == nil {
		return nil
	}
	for index := range o.members {
		if o.members[index].key == key {
			return o.members[index].value
		}
	}
	return nil
}

// has reports whether the object stores a value under key.
func (o *jsonObject) has(key string) bool {
	if o == nil {
		return false
	}
	for index := range o.members {
		if o.members[index].key == key {
			return true
		}
	}
	return false
}

// set stores value under key. A repeated name replaces the earlier member, as
// decoding into a map would keep the last one.
func (o *jsonObject) set(key string, value any) {
	for index := range o.members {
		if o.members[index].key == key {
			o.members[index].value = value
			return
		}
	}
	o.members = append(o.members, jsonMember{key: key, value: value})
}

// MarshalJSON encodes the object with its members in document order, so a
// parsed subtree can be sent back as a request payload.
func (o *jsonObject) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte('{')
	for index, member := range o.members {
		if index > 0 {
			buffer.WriteByte(',')
		}
		key, err := json.Marshal(member.key)
		if err != nil {
			return nil, err
		}
		buffer.Write(key)
		buffer.WriteByte(':')
		value, err := json.Marshal(member.value)
		if err != nil {
			return nil, err
		}
		buffer.Write(value)
	}
	buffer.WriteByte('}')
	return buffer.Bytes(), nil
}

// decodeResponse parses a response into the ordered form the extractors walk.
// It gives nil, which they read as empty, for what is not JSON.
func decodeResponse(raw json.RawMessage) any {
	parser := jsonParser{
		decoder: jsontext.NewDecoder(bytes.NewReader(raw), jsontext.AllowDuplicateNames(true)),
		keys:    make(map[string]string, 64),
	}
	value, err := parser.read()
	if err != nil {
		return nil
	}
	return value
}

// jsonParser builds the ordered form from a decoder. It holds what a parse
// shares across all of its objects and arrays, so each of those costs one
// exact-size allocation instead of a run of growing ones.
type jsonParser struct {
	decoder *jsontext.Decoder
	// keys interns member names. A response repeats a few hundred names
	// ("runs", "text", "navigationEndpoint"...) tens of thousands of times,
	// and a name is only ever compared, never changed.
	keys map[string]string
	// members and elements are stacks of the objects and arrays being read:
	// an object's members sit above its parent's until it ends, and are then
	// copied out at their exact count.
	members  []jsonMember
	elements []any
}

// read reads one JSON value: an object, an array, or a scalar.
func (p *jsonParser) read() (any, error) {
	switch p.decoder.PeekKind() {
	case jsontext.KindBeginObject:
		return p.readObject()
	case jsontext.KindBeginArray:
		return p.readArray()
	default:
		token, err := p.decoder.ReadToken()
		if err != nil {
			return nil, err
		}
		switch token.Kind() {
		case jsontext.KindNull:
			return nil, nil
		case jsontext.KindTrue:
			return true, nil
		case jsontext.KindFalse:
			return false, nil
		case jsontext.KindString:
			return token.String(), nil
		default:
			number, err := token.Float()
			if err != nil {
				return nil, err
			}
			return number, nil
		}
	}
}

// readKey reads a member name, sharing one string between every member that
// has it.
func (p *jsonParser) readKey() (string, error) {
	raw, err := p.decoder.ReadValue()
	if err != nil {
		return "", err
	}
	quoted := []byte(raw)
	if bytes.IndexByte(quoted, '\\') >= 0 {
		name, err := jsontext.AppendUnquote(nil, raw)
		if err != nil {
			return "", err
		}
		return string(name), nil
	}
	name := quoted[1 : len(quoted)-1]
	if key, ok := p.keys[string(name)]; ok {
		return key, nil
	}
	key := string(name)
	p.keys[key] = key
	return key, nil
}

func (p *jsonParser) readObject() (*jsonObject, error) {
	if _, err := p.decoder.ReadToken(); err != nil { // '{'
		return nil, err
	}
	base := len(p.members)
	for p.decoder.PeekKind() != jsontext.KindEndObject {
		key, err := p.readKey()
		if err != nil {
			return nil, err
		}
		value, err := p.read()
		if err != nil {
			return nil, err
		}
		// A repeated name replaces the earlier member, as decoding into a
		// map would keep the last one.
		replaced := false
		for index := base; index < len(p.members); index++ {
			if p.members[index].key == key {
				p.members[index].value = value
				replaced = true
				break
			}
		}
		if !replaced {
			p.members = append(p.members, jsonMember{key: key, value: value})
		}
	}
	if _, err := p.decoder.ReadToken(); err != nil { // '}'
		return nil, err
	}
	object := &jsonObject{members: make([]jsonMember, len(p.members)-base)}
	copy(object.members, p.members[base:])
	clear(p.members[base:])
	p.members = p.members[:base]
	return object, nil
}

func (p *jsonParser) readArray() ([]any, error) {
	if _, err := p.decoder.ReadToken(); err != nil { // '['
		return nil, err
	}
	base := len(p.elements)
	for p.decoder.PeekKind() != jsontext.KindEndArray {
		value, err := p.read()
		if err != nil {
			return nil, err
		}
		p.elements = append(p.elements, value)
	}
	if _, err := p.decoder.ReadToken(); err != nil { // ']'
		return nil, err
	}
	array := make([]any, len(p.elements)-base)
	copy(array, p.elements[base:])
	clear(p.elements[base:])
	p.elements = p.elements[:base]
	return array, nil
}
