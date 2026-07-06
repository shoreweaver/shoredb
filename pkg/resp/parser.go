package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

const (
	SimpleString = '+'
	BulkString   = '$'
	Array        = '*'
	Integer      = ':'
	SimpleError  = '-'
	Null         = '_'
)

type Value struct {
	Type  byte
	Str   string
	Num   int
	Array []Value
}

type Parser struct {
	reader *bufio.Reader
}

func NewParser(r io.Reader) *Parser {
	return &Parser{reader: bufio.NewReader(r)}
}

func (p *Parser) Parse() (Value, error) {
	prefix, err := p.reader.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch prefix {
	case SimpleString:
		return p.readSimpleString()
	case BulkString:
		return p.readBulkString()
	case Array:
		return p.readArray()
	case Integer:
		return p.readInt()
	case SimpleError:
		return p.readError()
	default:
		return Value{}, fmt.Errorf("unknown RESP type: %c", prefix)
	}
}

// reads a line until CRLF and removes the CRLF
func (p *Parser) readLine() ([]byte, error) {
	line, err := p.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 2 {
		return nil, fmt.Errorf("invalid RESP line: too short")
	}
	return line[:len(line)-2], nil // removes \r\n
}

func (p *Parser) readInt() (Value, error) {
	line, err := p.readLine()
	if err != nil {
		return Value{}, err
	}

	num, err := strconv.Atoi(string(line))
	if err != nil {
		return Value{}, fmt.Errorf("invalid integer: %w", err)
	}

	return Value{Type: Integer, Num: num}, nil
}

func (p *Parser) readSimpleString() (Value, error) {
	line, err := p.readLine()
	if err != nil {
		return Value{}, err
	}
	return Value{Type: SimpleString, Str: string(line)}, nil
}

func (p *Parser) readError() (Value, error) {
	line, err := p.readLine()
	if err != nil {
		return Value{}, err
	}
	return Value{Type: SimpleError, Str: string(line)}, nil
}

func (p *Parser) readBulkString() (Value, error) {
	line, err := p.readLine()
	if err != nil {
		return Value{}, err
	}

	size, err := strconv.Atoi(string(line))
	if err != nil {
		return Value{}, fmt.Errorf("invalid bulk string length: %w", err)
	}

	// null bulk string
	if size == -1 {
		return Value{Type: Null}, nil
	}

	buf := make([]byte, size)
	if _, err := io.ReadFull(p.reader, buf); err != nil {
		return Value{}, err
	}

	// Read and discard CRLF
	if _, err := p.reader.ReadBytes('\n'); err != nil {
		return Value{}, err
	}

	return Value{Type: BulkString, Str: string(buf)}, nil
}

func (p *Parser) readArray() (Value, error) {
	line, err := p.readLine()
	if err != nil {
		return Value{}, err
	}

	length, err := strconv.Atoi(string(line))
	if err != nil {
		return Value{}, fmt.Errorf("invalid array length: %w", err)
	}

	if length == -1 {
		return Value{Type: Null}, nil
	}

	arr := make([]Value, length)
	for i := 0; i < length; i++ {
		value, err := p.Parse()
		if err != nil {
			return Value{}, err
		}
		arr[i] = value
	}

	return Value{Type: Array, Array: arr}, nil
}

// Writer (RESP protocol writing)
type Writer struct {
	writer *bufio.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{writer: bufio.NewWriter(w)}
}

func (w *Writer) writeSimpleString(s string) error {
	w.writer.WriteByte(SimpleString)
	w.writer.WriteString(s)
	_, err := w.writer.WriteString("\r\n")
	return err
}

func (w *Writer) writeInt(i int) error {
	w.writer.WriteByte(Integer)
	w.writer.WriteString(strconv.Itoa(i))
	_, err := w.writer.WriteString("\r\n")
	return err
}

func (w *Writer) writeBulkString(s string) error {
	w.writer.WriteByte(BulkString)
	w.writer.WriteString(strconv.Itoa(len(s)))
	w.writer.WriteString("\r\n")
	w.writer.WriteString(s)
	_, err := w.writer.WriteString("\r\n")
	return err
}

func (w *Writer) writeSimpleError(s string) error {
	w.writer.WriteByte(SimpleError)
	w.writer.WriteString(s)
	_, err := w.writer.WriteString("\r\n")
	return err
}

func (w *Writer) writeArray(arr []Value) error {
	w.writer.WriteByte(Array)
	w.writer.WriteString(strconv.Itoa(len(arr)))
	w.writer.WriteString("\r\n")

	for _, v := range arr {
		if err := w.Write(v); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) writeNull() error {
	_, err := w.writer.WriteString("$-1\r\n")
	return err
}

func (w *Writer) Write(v Value) error {
	var err error

	switch v.Type {
	case SimpleString:
		err = w.writeSimpleString(v.Str)
	case SimpleError:
		err = w.writeSimpleError(v.Str)
	case Integer:
		err = w.writeInt(v.Num)
	case BulkString:
		err = w.writeBulkString(v.Str)
	case Array:
		err = w.writeArray(v.Array)
	case Null:
		err = w.writeNull()
	default:
		return fmt.Errorf("unknown RESP type for serialization: %c", v.Type)
	}

	if err != nil {
		return err
	}
	return w.writer.Flush()
}
