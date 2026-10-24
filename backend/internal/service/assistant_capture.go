package service

import (
	"bytes"
	"io"
	"strings"
)

// maxCaptureBytes — предел копии ответа агента для разбора памяти.
const maxCaptureBytes = 1 << 20 // 1 МБ

type captureBody struct {
	src        io.ReadCloser
	stream     bool
	onEnvelope func(raw []byte)
	closed     bool

	// JSON-режим (/v1/ask): копим всё тело.
	buf    bytes.Buffer
	eof    bool
	tooBig bool

	// SSE-режим (/v1/ask/stream): ищем кадр event: done.
	sse sseCapture
}

// newCaptureBody оборачивает src.
func newCaptureBody(src io.ReadCloser, stream bool, onEnvelope func(raw []byte)) *captureBody {
	return &captureBody{src: src, stream: stream, onEnvelope: onEnvelope}
}

func (c *captureBody) Read(p []byte) (int, error) {
	n, err := c.src.Read(p)
	if n > 0 {
		c.capture(p[:n])
	}
	if err == io.EOF {
		c.eof = true
	}
	return n, err
}

// Close закрывает исходное тело и единожды вызывает onEnvelope.
func (c *captureBody) Close() error {
	err := c.src.Close()
	if !c.closed {
		c.closed = true
		c.emit()
	}
	return err
}

func (c *captureBody) capture(b []byte) {
	if c.stream {
		c.sse.feed(b)
		return
	}
	if c.tooBig {
		return
	}
	if c.buf.Len()+len(b) > maxCaptureBytes {
		c.tooBig = true
		c.buf.Reset()
		return
	}
	c.buf.Write(b)
}

func (c *captureBody) emit() {
	if c.onEnvelope == nil {
		return
	}
	var raw []byte
	if c.stream {
		raw = c.sse.doneData()
	} else if c.eof && !c.tooBig {
		raw = c.buf.Bytes()
	}
	if len(raw) > 0 {
		c.onEnvelope(raw)
	}
}

type sseCapture struct {
	line     bytes.Buffer
	curEvent string
	curData  bytes.Buffer
	overflow bool
	done     []byte
}

func (s *sseCapture) feed(b []byte) {
	for _, ch := range b {
		if ch == '\n' {
			s.lineDone()
			continue
		}
		s.line.WriteByte(ch)
	}
}

func (s *sseCapture) lineDone() {
	line := strings.TrimSuffix(s.line.String(), "\r")
	s.line.Reset()

	switch {
	case line == "":
		if s.curEvent == "done" && !s.overflow {
			s.done = append([]byte(nil), s.curData.Bytes()...)
		}
		s.curEvent = ""
		s.curData.Reset()
		s.overflow = false
	case strings.HasPrefix(line, "event:"):
		s.curEvent = strings.TrimSpace(line[len("event:"):])
	case strings.HasPrefix(line, "data:"):
		val := strings.TrimPrefix(line[len("data:"):], " ")
		extra := len(val)
		if s.curData.Len() > 0 {
			extra++
		}
		if s.curData.Len()+extra > maxCaptureBytes {
			s.overflow = true
			return
		}
		if s.curData.Len() > 0 {
			s.curData.WriteByte('\n')
		}
		s.curData.WriteString(val)
	}
}

// doneData возвращает data последнего завершённого кадра done (nil, если его не было).
func (s *sseCapture) doneData() []byte { return s.done }
