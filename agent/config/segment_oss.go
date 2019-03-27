// +build !ent

package config

func (b *Builder) validateSegments(rt RuntimeConfig) error {
	// if rt.SegmentName != "" {
	// 	return structs.ErrSegmentsNotSupported
	// }
	// if len(rt.Segments) > 0 {
	// 	return structs.ErrSegmentsNotSupported
	// }
	return nil
}
