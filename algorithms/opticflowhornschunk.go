/*
HornSchunk computes an optic flow field using the method of
Horn & Schunk and an iterative Jacobi scheme
*/
package algorithms

import (
	"flag"
	"github.com/niklas88/imgtest/floatimage"
	"math"
	"sync"
)

var numRowsPerGo int

// Derivatives fx, fy, fz as 3 channel FloatImg to make access sane
const (
	Fxc = iota
	Fyc = iota
	Fzc = iota
)

func init() {
	flag.IntVar(&numRowsPerGo, "rowspergo", 1, "Number of rows to compute per Goroutine")
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}

func innerDerive(f1, f2, derivs *floatimage.FloatImg, wg *sync.WaitGroup, minRow, maxRow int) {
	const hx = 1.0
	const hy = 1.0
	bounds := f1.Bounds()
	for j := minRow; j < maxRow; j++ {
		for i := bounds.Min.X + 1; i < bounds.Max.X-1; i++ {
			Fx := (f1.AtF(i+1, j)[0] - f1.AtF(i-1, j)[0] + f2.AtF(i+1, j)[0] - f2.AtF(i-1, j)[0]) / (4.0 * hx)
			Fy := (f1.AtF(i, j+1)[0] - f1.AtF(i, j-1)[0] + f2.AtF(i, j+1)[0] - f2.AtF(i, j-1)[0]) / (4.0 * hy)
			Fz := f2.AtF(i, j)[0] - f1.AtF(i, j)[0]
			dvs := derivs.AtF(i, j)
			dvs[Fxc], dvs[Fyc], dvs[Fzc] = Fx, Fy, Fz
		}
	}
	wg.Done()
}

func deriveMixed(f1, f2 *floatimage.FloatImg) *floatimage.FloatImg {
	const hx = 1.0
	const hy = 1.0
	var wg sync.WaitGroup
	bounds := f1.Bounds()
	derivs := floatimage.NewFloatImg(bounds, 3)
	for lower := bounds.Min.Y + 1; lower < bounds.Max.Y-1; {
		upper := min(lower+numRowsPerGo, bounds.Max.Y-1)
		wg.Add(1)
		go innerDerive(f1, f2, derivs, &wg, lower, upper)
		lower = upper
	}
	wg.Wait()
	return derivs
}

func innerFlow(alpha float32, derivs, oldvec, vecField *floatimage.FloatImg, wg *sync.WaitGroup, minRow, maxRow int) {
	bounds := vecField.Bounds()

	help := 1.0 / alpha
	var nn int
	var uSum, vSum float32
	var uv []float32
	for j := minRow; j < maxRow; j++ {
		for i := bounds.Min.X; i < bounds.Max.X; i++ {
			nn = 0
			uSum, vSum = 0, 0
			if i > bounds.Min.X {
				nn++
				uv = oldvec.AtF(i-1, j)
				uSum += uv[0]
				vSum += uv[1]
			}

			if i < bounds.Max.X-1 {
				nn++
				uv = oldvec.AtF(i+1, j)
				uSum += uv[0]
				vSum += uv[1]
			}

			if j > bounds.Min.Y {
				nn++
				uv = oldvec.AtF(i, j-1)
				uSum += uv[0]
				vSum += uv[1]
			}

			if j < bounds.Max.Y-1 {
				nn++
				uv = oldvec.AtF(i, j+1)
				uSum += uv[0]
				vSum += uv[1]
			}
			dvs := derivs.AtF(i, j)
			fxij, fyij, fzij := dvs[Fxc], dvs[Fyc], dvs[Fzc]
			uv = oldvec.AtF(i, j)
			uSum -= help * fxij * (fyij*uv[1] + fzij)
			uSum /= float32(nn) + help*fxij*fxij
			vSum -= help * fyij * (fxij*uv[0] + fzij)
			vSum /= float32(nn) + help*fyij*fyij
			uv = vecField.AtF(i, j)
			uv[0], uv[1] = uSum, vSum
		}
	}
	wg.Done()
}

func flow(alpha float32, derivs, oldvec, vecField *floatimage.FloatImg) {
	bounds := vecField.Bounds()
	var wg sync.WaitGroup

	for lower := bounds.Min.Y; lower < bounds.Max.Y;{
		upper := min(lower + numRowsPerGo, bounds.Max.Y)
		wg.Add(1)
		go innerFlow(alpha, derivs, oldvec, vecField, &wg, lower, upper)
		lower = upper
	}
	wg.Wait()
}

// OpticFlowHornSchunk computes the optic flow between two images
// the images need to have Dummie borders (see floatimage.Dummies())
// applied.
// It returns the optic flow field as a 2 channel floatimage.FloatImg
func OpticFlowHornSchunk(f1, f2 *floatimage.FloatImg, alpha float32, iterations int) (uv *floatimage.FloatImg) {
	// Compute fx, fy, fz derivatives as FloatImg with 3 channels for faster access
	derivs := deriveMixed(f1, f2)
	// bounds without dummies
	bounds := f1.Bounds()

	// vector field as FloatImg with 2 channels
	uv = floatimage.NewFloatImg(bounds, 2)
	// temporary storage for vector field from previous iteration
	uvOld := floatimage.NewFloatImg(bounds, 2)
	// Process image using the Jacobi method to incrementally compute the vector field
	for k := 1; k <= iterations; k++ {
		flow(float32(alpha), derivs, uvOld, uv)
		uvOld.Copy(uv)
	}

	return
}

// MagImage generates a magnitude image from an optic flow
// field and returns it as a single channel floatimage.FloatImg
func MagImage(uv *floatimage.FloatImg) (magImg *floatimage.FloatImg) {
	bounds := uv.Bounds()
	// Magnitude image
	magImg = floatimage.NewFloatImg(bounds, 1)
	// Calculate
	var wg sync.WaitGroup
	wg.Add(bounds.Max.Y - bounds.Min.Y)
	for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
		go func(j int) {
			for i := bounds.Min.X; i < bounds.Max.X; i++ {
				vec := uv.AtF(i, j)
				tmp := vec[0]*vec[0] + vec[1]*vec[1]
				magImg.Set(i, j, 0, float32(math.Sqrt(float64(tmp))))
			}
			wg.Done()
		}(j)
	}
	wg.Wait()
	return

}
