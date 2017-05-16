// Copyright 2017 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tsdb

import (
	"math/rand"
	"testing"

	"github.com/pkg/errors"
	"github.com/prometheus/tsdb/chunks"
	"github.com/stretchr/testify/require"
)

type mockChunkReader map[uint64]chunks.Chunk

func (cr mockChunkReader) Chunk(ref uint64) (chunks.Chunk, error) {
	chk, ok := cr[ref]
	if ok {
		return chk, nil
	}

	return nil, errors.New("Chunk with ref not found")
}

func (cr mockChunkReader) Close() error {
	return nil
}

func TestAddingNewIntervals(t *testing.T) {
	cases := []struct {
		exist []trange
		new   trange

		exp []trange
	}{
		{
			new: trange{1, 2},
			exp: []trange{{1, 2}},
		},
		{
			exist: []trange{{1, 10}, {12, 20}, {25, 30}},
			new:   trange{21, 23},
			exp:   []trange{{1, 10}, {12, 20}, {21, 23}, {25, 30}},
		},
		{
			exist: []trange{{1, 10}, {12, 20}, {25, 30}},
			new:   trange{21, 25},
			exp:   []trange{{1, 10}, {12, 20}, {21, 30}},
		},
		{
			exist: []trange{{1, 10}, {12, 20}, {25, 30}},
			new:   trange{18, 23},
			exp:   []trange{{1, 10}, {12, 23}, {25, 30}},
		},
		// TODO(gouthamve): (below) This is technically right, but fix it in the future.
		{
			exist: []trange{{1, 10}, {12, 20}, {25, 30}},
			new:   trange{9, 23},
			exp:   []trange{{1, 23}, {12, 20}, {25, 30}},
		},
		{
			exist: []trange{{5, 10}, {12, 20}, {25, 30}},
			new:   trange{1, 4},
			exp:   []trange{{1, 4}, {5, 10}, {12, 20}, {25, 30}},
		},
	}

	for _, c := range cases {
		require.Equal(t, c.exp, addNewInterval(c.exist, c.new))
	}
	return
}

func TestDeletedIterator(t *testing.T) {
	chk := chunks.NewXORChunk()
	app, err := chk.Appender()
	require.NoError(t, err)
	// Insert random stuff from (0, 1000).
	act := make([]sample, 1000)
	for i := 0; i < 1000; i++ {
		act[i].t = int64(i)
		act[i].v = rand.Float64()
		app.Append(act[i].t, act[i].v)
	}

	cases := []struct {
		r []trange
	}{
		{r: []trange{{1, 20}}},
		{r: []trange{{1, 10}, {12, 20}, {21, 23}, {25, 30}}},
		{r: []trange{{1, 10}, {12, 20}, {20, 30}}},
		{r: []trange{{1, 10}, {12, 23}, {25, 30}}},
		{r: []trange{{1, 23}, {12, 20}, {25, 30}}},
		{r: []trange{{1, 23}, {12, 20}, {25, 3000}}},
		{r: []trange{{0, 2000}}},
		{r: []trange{{500, 2000}}},
		{r: []trange{{0, 200}}},
		{r: []trange{{1000, 20000}}},
	}

	for _, c := range cases {
		i := int64(-1)
		it := &deletedIterator{it: chk.Iterator(), dranges: c.r[:]}
		ranges := c.r[:]
		for it.Next() {
			i++
			for _, tr := range ranges {
				if tr.inBounds(i) {
					i = tr.maxt + 1
					ranges = ranges[1:]
				}
			}

			require.True(t, i < 1000)

			ts, v := it.At()
			require.Equal(t, act[i].t, ts)
			require.Equal(t, act[i].v, v)
		}
		// There has been an extra call to Next().
		i++
		for _, tr := range ranges {
			if tr.inBounds(i) {
				i = tr.maxt + 1
				ranges = ranges[1:]
			}
		}

		require.False(t, i < 1000)
		require.NoError(t, it.Err())
	}
}
