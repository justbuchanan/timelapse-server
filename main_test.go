package main

import (
	"github.com/stretchr/testify/assert"
	"sort"
	"testing"
	"time"
)

func TestCalculateImageBrightness(t *testing.T) {
	brightness, err := CalculateImageBrightness("example/1500872173-image.jpg")
	if err != nil {
		t.Fatal("Error calculating image brightness ", err)
	}
	assert.Equalf(t, 0.36, brightness, "Check brightness value")
}

func TestProcessFiles(t *testing.T) {
	result := ReadImageFileInfos("./example", make([]time.Time, 0))
	assert.Equal(t, 31, len(result))

	if !sort.IsSorted(result) {
		t.Fatal("Result isn't sorted")
	}
}

func TestFilterAndGroupByDay(t *testing.T) {
	imgInfos := ReadImageFileInfos("./example", make([]time.Time, 0))
	grouped := FilterAndGroupByDay(imgInfos)

	assert.Equal(t, 2, len(grouped), "The photos were taken across two days")
}

func TestParseDate(t *testing.T) {
	str := "2017-07-23"
	tm, err := ParseDate(str)
	assert.Nil(t, err)
	assert.Equal(t, 2017, tm.Year())
	assert.Equal(t, 7, int(tm.Month()))
	assert.Equal(t, 23, tm.Day())
}
