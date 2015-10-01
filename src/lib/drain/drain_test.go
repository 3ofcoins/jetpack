package drain

import (
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"
)

var jabberwocky = []byte(`’Twas brillig, and the slithy toves
Did gyre and gimble in the wabe:
All mimsy were the borogoves,
And the mome raths outgrabe.

“Beware the Jabberwock, my son!
The jaws that bite, the claws that catch!
Beware the Jubjub bird, and shun
The frumious Bandersnatch!”

He took his vorpal sword in hand;
Long time the manxome foe he sought—
So rested he by the Tumtum tree
And stood awhile in thought.

And, as in uffish thought he stood,
The Jabberwock, with eyes of flame,
Came whiffling through the tulgey wood,
And burbled as it came!

One, two! One, two! And through and through
The vorpal blade went snicker-snack!
He left it dead, and with its head
He went galumphing back.

“And hast thou slain the Jabberwock?
Come to my arms, my beamish boy!
O frabjous day! Callooh! Callay!”
He chortled in his joy.

’Twas brillig, and the slithy toves
Did gyre and gimble in the wabe:
All mimsy were the borogoves,
And the mome raths outgrabe.
`)

func TestDrainWithOneWriter(t *testing.T) {
	d := make(Drain, 128) // enough buffer to avoid having to use goroutines
	w := d.NewWriter()
	for i := 0; i < len(jabberwocky); i += 128 {
		j := i + 128
		if j > len(jabberwocky) {
			j = len(jabberwocky)
		}
		w.Write(jabberwocky[i:j])
	}
	w.Close()
	close(d)
	ll := d.Lines()

	if len(ll) != 34 {
		t.Fatal("Wrong number of lines: expected 34, got", len(ll))
	}
}

func TestDrainWithManyWriters(t *testing.T) {
	bufferSizes := []int{
		3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37, 41, 43, 47, 53, 59, 61,
		67, 71, 73, 79, 83, 89, 97, 101, 103, 107, 109, 113, 127, 131,
		137, 139, 149, 151, 157, 163, 167, 173, 179, 181, 191, 193, 197,
		199, 211, 223, 227, 229, 233, 239, 241, 251, 257, 263, 269, 271,
		277, 281, 283, 293, 307, 311, 313, 317, 331, 337, 347, 349, 353,
		359, 367, 373, 379, 383, 389, 397, 401, 409, 419, 421, 431, 433,
		439, 443, 449, 457, 461, 463, 467,
	}

	d := make(Drain)
	wg := sync.WaitGroup{}
	for _, bs := range bufferSizes {
		wg.Add(1)
		go func(bs int) {
			defer wg.Done()
			w := d.NewWriter()
			for i := 0; i < len(jabberwocky); i += bs {
				j := i + bs
				if j > len(jabberwocky) {
					j = len(jabberwocky)
				}
				time.Sleep(time.Duration(rand.Int()%bs) * time.Millisecond)
				w.Write(jabberwocky[i:j])
			}
			w.Close()
		}(bs)
	}

	var ll []Line
	gotLines := make(chan int)
	go func() {
		ll = d.Lines()
		gotLines <- 1
	}()

	wg.Wait()
	close(d)
	<-gotLines

	res := make(map[*Writer][]string)

	for _, ln := range ll {
		res[ln.Writer] = append(res[ln.Writer], ln.Text)
	}

	if len(res) != len(bufferSizes) {
		t.Errorf("Wrong # of writers: expected %d, got %d", len(res), len(bufferSizes))
	}

	for _, single := range res {
		if len(single) != 34 {
			t.Errorf("Length of one of results not 34, but %d\n", len(single))
		}
		if glued := strings.Join(single, "\n") + "\n"; glued != string(jabberwocky) {
			t.Errorf("One of the results is not Jabberwocky: %#v\n", glued)
		}
	}
}
