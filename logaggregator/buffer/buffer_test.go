package buffer

import (
	"fmt"
	"testing"
	"time"

	. "github.com/flynn/flynn/Godeps/_workspace/src/github.com/flynn/go-check"
	"github.com/flynn/flynn/pkg/syslog/rfc5424"
)

// Hook gocheck up to the "go test" runner
func Test(t *testing.T) { TestingT(t) }

type S struct {
	data []*rfc5424.Message
}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	hdr := &rfc5424.Header{}

	s.data = make([]*rfc5424.Message, DefaultCapacity*2)
	for i := 0; i < len(s.data); i++ {
		line := []byte(fmt.Sprintf("line %d\n", i))
		s.data[i] = rfc5424.NewMessage(hdr, line)
	}
}

func (s *S) TestRead(c *C) {
	tests := []struct {
		cap        int
		data, want []*rfc5424.Message
	}{
		// fill
		{
			cap:  100,
			data: s.data[:100],
			want: s.data[:100],
		},
		// overflow
		{
			cap:  90,
			data: s.data[:100],
			want: s.data[10:100],
		},
		// large overflow
		{
			cap:  DefaultCapacity,
			data: s.data,
			want: s.data[DefaultCapacity:],
		},
	}

	for _, test := range tests {
		b := newBuffer(test.cap)
		defer b.Close()

		for _, msg := range test.data {
			c.Assert(b.Add(msg), IsNil)
		}

		got := b.Read()
		c.Assert(len(got), Equals, len(test.want))
		c.Assert(got, DeepEquals, test.want)
	}
}

type subscriber struct {
	msgc chan *rfc5424.Message
	want []*rfc5424.Message
}

func (s *S) TestSubscribe(c *C) {
	tests := []struct {
		data []*rfc5424.Message

		subs map[int][]subscriber
	}{
		// single subscriber from step 0
		{
			data: s.data,
			subs: map[int][]subscriber{
				0: {
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)),
						want: s.data,
					},
				},
			},
		},
		// multiple subscriber from step 0
		{
			data: s.data,
			subs: map[int][]subscriber{
				0: {
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)),
						want: s.data,
					},
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)),
						want: s.data,
					},
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)),
						want: s.data,
					},
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)),
						want: s.data,
					},
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)),
						want: s.data,
					},
				},
			},
		},
		// multiple subscribers, offset steps
		{
			data: s.data,
			subs: map[int][]subscriber{
				0: {
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)),
						want: s.data,
					},
				},
				100: {
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)-100),
						want: s.data[100:],
					},
				},
				200: {
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)-200),
						want: s.data[200:],
					},
				},
				300: {
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)-300),
						want: s.data[300:],
					},
				},
				400: {
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)-400),
						want: s.data[400:],
					},
				},
				500: {
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)-500),
						want: s.data[500:],
					},
				},
			},
		},
		// subscribers with various buffered channels sizes
		{
			data: s.data,
			subs: map[int][]subscriber{
				0: {
					subscriber{
						msgc: make(chan *rfc5424.Message),
						want: s.data[:0],
					},
					subscriber{
						msgc: make(chan *rfc5424.Message, 1),
						want: s.data[:1],
					},
					subscriber{
						msgc: make(chan *rfc5424.Message, 10),
						want: s.data[:10],
					},
					subscriber{
						msgc: make(chan *rfc5424.Message, 100),
						want: s.data[:100],
					},
					subscriber{
						msgc: make(chan *rfc5424.Message, 1000),
						want: s.data[:1000],
					},
					subscriber{
						msgc: make(chan *rfc5424.Message, len(s.data)),
						want: s.data,
					},
				},
			},
		},
	}

	for _, test := range tests {
		b := NewBuffer()

		donec := make(chan struct{})
		for step, msg := range test.data {
			for _, sub := range test.subs[step] {
				b.Subscribe(sub.msgc, donec)
			}

			c.Assert(b.Add(msg), IsNil)
		}

		b.Close()

		for _, subs := range test.subs {
			for _, sub := range subs {
				got := make([]*rfc5424.Message, 0, len(sub.msgc))
				for msg := range sub.msgc {
					got = append(got, msg)
				}

				c.Assert(len(got), Equals, len(sub.want))
				c.Assert(got, DeepEquals, sub.want)
			}
		}
	}
}

func (s *S) TestReadSubscribe(c *C) {
	tests := []struct {
		cap, subAt int
		data, want []*rfc5424.Message
		msgc       chan *rfc5424.Message
	}{
		// read 1/4, sub 3/4 (cap is 1/2)
		{
			cap:   500,
			data:  s.data[:1000],
			subAt: 250,
			msgc:  make(chan *rfc5424.Message, 750),
			want:  s.data[:1000],
		},
		// drop 1/4, read 1/2, sub 1/4 (cap is 1/2)
		{
			cap:   500,
			data:  s.data[:1000],
			subAt: 750,
			msgc:  make(chan *rfc5424.Message, 250),
			want:  s.data[250:1000],
		},
		// read 1/2, sub 1/4, drop 1/4 (len(msgc) is 1/4)
		{
			cap:   1000,
			data:  s.data[:1000],
			subAt: 500,
			msgc:  make(chan *rfc5424.Message, 250),
			want:  s.data[:750],
		},
	}

	for _, test := range tests {
		var got []*rfc5424.Message

		b := newBuffer(test.cap)

		donec := make(chan struct{})
		for step, msg := range test.data {
			if step == test.subAt {
				got = b.ReadAndSubscribe(test.msgc, donec)
			}

			c.Assert(b.Add(msg), IsNil)
		}

		b.Close()

		for msg := range test.msgc {
			got = append(got, msg)
		}

		c.Assert(len(got), Equals, len(test.want))
		c.Assert(got, DeepEquals, test.want)
	}
}

func (s *S) TestAddSort(c *C) {
	b := NewBuffer()

	// add first message
	first := rfc5424.NewMessage(nil, []byte("msg1"))
	b.Add(first)
	c.Assert(b.Read(), DeepEquals, []*rfc5424.Message{first})

	// add new, before head
	newHead := rfc5424.NewMessage(nil, []byte("msg2"))
	newHead.Timestamp = first.Timestamp.Add(-time.Second)
	b.Add(newHead)
	c.Assert(b.Read(), DeepEquals, []*rfc5424.Message{newHead, first})

	// add new, tail
	newTail := rfc5424.NewMessage(nil, []byte("msg3"))
	newTail.Timestamp = first.Timestamp.Add(2 * time.Second)
	b.Add(newTail)
	c.Assert(b.Read(), DeepEquals, []*rfc5424.Message{newHead, first, newTail})

	// add new between first and newTail
	between := rfc5424.NewMessage(nil, []byte("msg3"))
	between.Timestamp = first.Timestamp.Add(time.Second)
	b.Add(between)
	c.Assert(b.Read(), DeepEquals, []*rfc5424.Message{newHead, first, between, newTail})
}
