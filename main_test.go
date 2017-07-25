package main

import (
	"github.com/stretchr/testify/assert"
	"sort"
	"testing"
)

func TestCalculateImageBrightness(t *testing.T) {
	brightness, err := CalculateImageBrightness("example/1500872173-image.jpg")
	if err != nil {
		t.Fatal("Error calculating image brightness ", err)
	}
	assert.Equalf(t, 0.36, brightness, "Check brightness value")
}

func TestProcessFiles(t *testing.T) {
	result := ReadImageFileInfos("./example")
	assert.Equal(t, 31, len(result))

	// assert.True(t, true, result[len(result)-1].Timestamp.After(result[0].Timestamp))
	if !sort.IsSorted(result) {
		t.Fatal("Result isn't sorted")
	}
}

func TestFilterAndGroupByDay(t *testing.T) {
	imgInfos := ReadImageFileInfos("./example")
	grouped := FilterAndGroupByDay(imgInfos)

	assert.Equal(t, 2, len(grouped), "The photos were taken across two days")
}
