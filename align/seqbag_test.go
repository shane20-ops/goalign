package align

import (
	"reflect"
	"testing"
)

func Test_seqbag_UniqueCharacters(t *testing.T) {
	type fields struct {
		seqmap          map[string]*seq
		seqs            []*seq
		ignoreidentical bool
		alphabet        int
	}
	tests := []struct {
		name      string
		fields    fields
		wantChars []rune
	}{
		{name: "t1",
			fields: fields{seqmap: nil,
				seqs: []*seq{
					&seq{sequence: []rune("ACGTACGTACGT")},
					&seq{sequence: []rune("ACGTAC*TACGT")}}},
			wantChars: []rune("*ACGT")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := &seqbag{
				seqmap:          tt.fields.seqmap,
				seqs:            tt.fields.seqs,
				ignoreidentical: tt.fields.ignoreidentical,
				alphabet:        tt.fields.alphabet,
			}
			if gotChars := sb.UniqueCharacters(); !reflect.DeepEqual(gotChars, tt.wantChars) {
				t.Errorf("seqbag.UniqueCharacters() = %v, want %v", gotChars, tt.wantChars)
			}
		})
	}
}
