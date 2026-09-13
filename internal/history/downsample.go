package history

import "slices"

// MaxPoints คือจำนวนจุดสูงสุดที่ส่งให้ front-end ต่อหนึ่ง request (สเปกหัวข้อ 5)
const MaxPoints = 5000

// Downsample เลือก index ของจุดที่จะส่งออกจากทั้งหมด n จุด ให้ไม่เกิน limit
//
// หยิบทุก ๆ N จุด + จุดสุดท้าย + จุดใน keep (เช่นจุดที่ events ชี้อยู่)
// คืน index เรียงจากน้อยไปมากและไม่ซ้ำ ถ้า keep มีมากกว่า limit เอง ผลจะเกิน limit เพราะ keep ต้องชนะ
func Downsample(n, limit int, keep []int) []int {
	if n <= 0 {
		return []int{}
	}
	if n <= limit {
		all := make([]int, n)
		for i := range all {
			all[i] = i
		}
		return all
	}

	budget := max(1, limit-len(keep)-1)
	stride := (n + budget - 1) / budget // ปัดขึ้น จำนวนที่หยิบจะไม่เกิน budget

	picked := make([]int, 0, budget+len(keep)+1)
	for i := 0; i < n; i += stride {
		picked = append(picked, i)
	}
	picked = append(picked, n-1)
	for _, index := range keep {
		if index >= 0 && index < n {
			picked = append(picked, index)
		}
	}

	slices.Sort(picked)
	return slices.Compact(picked)
}
