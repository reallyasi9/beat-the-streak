package main

type StringPermutator []string

func (s StringPermutator) Permute(c chan<- []string) {
    count := make([]int, len(s))

    out := make([]string, len(s))
    copy(out, s)
    c <- out

    i := 0
    for i < len(s) {
        if count[i] < i {
            if i % 2 == 0 {
                out[0], out[i] = out[i], out[0]
            } else {
                out[count[i]], out[i] = out[i], out[count[i]]
            }
            c <- out
            count[i] += 1
            i = 0
        } else {
            count[i] = 0
            i += 1
        }
    }
}
