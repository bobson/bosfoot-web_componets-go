package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// ojson is an order-preserving JSON object, so the admin can edit data/*.json
// and write them back with the same key order and formatting (2-space indent,
// raw UTF-8 and "&") — git diffs then show only the real change.
type ojson struct {
	keys []string
	vals map[string]any // values: *ojson, []any, json.Number, string, bool, nil
}

func newOJSON() *ojson { return &ojson{vals: map[string]any{}} }

func (o *ojson) Get(k string) (any, bool) { v, ok := o.vals[k]; return v, ok }

// Set adds (at the end) or replaces a key.
func (o *ojson) Set(k string, v any) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func (o *ojson) Delete(k string) {
	if _, ok := o.vals[k]; !ok {
		return
	}
	delete(o.vals, k)
	for i, x := range o.keys {
		if x == k {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

// Obj returns the child object at k (nil if absent or not an object).
func (o *ojson) Obj(k string) *ojson {
	c, _ := o.vals[k].(*ojson)
	return c
}

func (o *ojson) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := marshalRaw(k) // keys too: a colour key like "White & Beige"
		b.Write(kb)
		b.WriteByte(':')
		vb, err := marshalRaw(o.vals[k])
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// marshalRaw encodes without HTML escaping (keeps "&" as-is, like the files).
func marshalRaw(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// parseOJSON decodes a document whose top level is an object.
func parseOJSON(data []byte) (*ojson, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	o, ok := v.(*ojson)
	if !ok {
		return nil, fmt.Errorf("top level is not a JSON object")
	}
	return o, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := newOJSON()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k, _ := kt.(string)
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				o.Set(k, v)
			}
			_, err := dec.Token() // '}'
			return o, err
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			_, err := dec.Token() // ']'
			return arr, err
		}
		return nil, fmt.Errorf("unexpected %v", t)
	default:
		return t, nil // string, json.Number, bool, nil
	}
}

// formatOJSON renders the file body: 2-space indent, no HTML escaping, final newline.
func formatOJSON(o *ojson) ([]byte, error) {
	raw, err := marshalRaw(o)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := json.Indent(&b, raw, "", "  "); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func readOJSON(path string) (*ojson, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseOJSON(data)
}

// writeOJSON writes atomically (temp file + rename) so a crash never leaves a
// half-written data file.
func writeOJSON(path string, o *ojson) error {
	body, err := formatOJSON(o)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
