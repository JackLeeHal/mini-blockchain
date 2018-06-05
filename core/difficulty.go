package core

import (
	"fmt"
	"math/big"
)

type Difficulty interface {
	ReachDifficulty(hash [HashSize]byte) bool
	UpdateDifficulty(usedTimeMs uint64) error
	Print() string
}

type SimpleDifficulty struct {
	targetBlockIntervalMs uint64 /* interval in ms */
	difficulty            [HashSize]byte
}

/*
 * Moving average difficulty algorithm
 * Note that BCH's new difficulty algorithm is similar to the algorithm with
 * - target interval is 10mins
 * - maSamples is 144
 * That is moving average over the difficulty in a day
 */
type MADifficulty struct {
	targetBlockIntervalMs uint64
	maSamples             uint32 /* number of samples to ma */
	workSamples           []*big.Int
	usedTimeMsSamples     []uint64 /* used time samples */
	difficulty            [HashSize]byte
}

func CreateSimpleDifficulty(targetBlockIntervalMs uint64, prob float64) Difficulty {
	var diff SimpleDifficulty
	diff.targetBlockIntervalMs = targetBlockIntervalMs
	var buf [HashSize]byte
	for i := range buf {
		buf[i] = byte(prob * 256)
		prob = prob*256 - float64(buf[i])
	}
	diff.difficulty = buf
	return &diff
}

func hashIsSmallerOrEqual(hash1 *[HashSize]byte, hash2 *[HashSize]byte) bool {
	for i := 0; i < HashSize; i++ {
		if hash1[i] < hash2[i] {
			break
		} else if hash1[i] > hash2[i] {
			return false
		}
	}
	return true
}

func (d *SimpleDifficulty) ReachDifficulty(hash [HashSize]byte) bool {
	return hashIsSmallerOrEqual(&hash, &d.difficulty)
}

func (d *SimpleDifficulty) UpdateDifficulty(usedTimeMs uint64) error {
	var v, target, used, mul, newDiff big.Int
	v.SetBytes(d.difficulty[:])
	target.SetUint64(d.targetBlockIntervalMs)
	used.SetUint64(usedTimeMs)
	mul.Mul(&v, &used)
	newDiff.Div(&mul, &target)

	buf := newDiff.Bytes()
	for i := range d.difficulty {
		d.difficulty[i] = 0
	}
	for i, b := range buf {
		d.difficulty[HashSize-len(buf)+i] = b
	}
	return nil
}

func (d *SimpleDifficulty) Print() string {
	return fmt.Sprintf("SimpleDifficulty:[targetBlockIntervalMs:%v,difficulty:%v] \n",
		d.targetBlockIntervalMs,
		d.difficulty,
	)
}

func CreateMADifficulty(targetBlockIntervalMs uint64, prob float64, maSamples uint32) Difficulty {
	var diff MADifficulty
	diff.targetBlockIntervalMs = targetBlockIntervalMs
	var buf [HashSize]byte
	for i := range buf {
		buf[i] = byte(prob * 256)
		prob = prob*256 - float64(buf[i])
	}
	diff.difficulty = buf
	diff.maSamples = maSamples
	return &diff
}

func (d *MADifficulty) ReachDifficulty(hash [HashSize]byte) bool {
	return hashIsSmallerOrEqual(&hash, &d.difficulty)
}

func diffToWork(diff [HashSize]byte) *big.Int {
	var unit [HashSize + 1]byte
	unit[0] = 1
	var uInt, dInt, wInt big.Int
	uInt.SetBytes(unit[:])
	dInt.SetBytes(diff[:])
	wInt.Div(&uInt, &dInt)
	return &wInt
}

func workToDiff(work *big.Int) *[HashSize]byte {
	var unit [HashSize + 1]byte
	unit[0] = 1
	var uInt, dInt big.Int
	uInt.SetBytes(unit[:])
	dInt.Div(&uInt, work)

	var diff [HashSize]byte
	buf := dInt.Bytes()

	for i, b := range buf {
		diff[HashSize-len(buf)+i] = b
	}
	return &diff
}

func (d *MADifficulty) UpdateDifficulty(usedTimeMs uint64) error {
	d.usedTimeMsSamples = append(d.usedTimeMsSamples, usedTimeMs)
	d.workSamples = append(d.workSamples, diffToWork(d.difficulty))

	if uint32(len(d.usedTimeMsSamples)) < d.maSamples {
		return nil
	}

	if uint32(len(d.usedTimeMsSamples)) > d.maSamples {
		d.usedTimeMsSamples = d.usedTimeMsSamples[1:]
		d.workSamples = d.workSamples[1:]
	}

	var totalWork big.Int
	var totalTimeMs uint64
	for _, w := range d.workSamples {
		currentWork := totalWork
		totalWork.Add(&currentWork, w)
	}

	for _, usedMs := range d.usedTimeMsSamples {
		totalTimeMs += usedMs
	}

	var expectedWork, used, target, tmp big.Int
	target.SetUint64(d.targetBlockIntervalMs)
	used.SetUint64(totalTimeMs)
	tmp.Mul(&totalWork, &target)
	expectedWork.Div(&tmp, &used)

	d.difficulty = *workToDiff(&expectedWork)

	return nil
}

func (d *MADifficulty) Print() string {
	return fmt.Sprintf("MADifficulty:[targetBlockIntervalMs:%v,maSamples:%d,workSamples:%v,usedTimeMsSamples:%v,difficulty:%v] \n",
		d.targetBlockIntervalMs,
		d.maSamples,
		d.workSamples,
		d.usedTimeMsSamples,
		d.difficulty,
	)
}