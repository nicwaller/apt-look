package apttransport

// TODO: implement an in-memory cache that retains a compressed version
//func compress(r io.Reader) ([]byte, error) {
//	buf := new(bytes.Buffer)
//	_, err := io.Copy(gzip.NewWriter(buf), r)
//	if err != nil {
//		return nil, err
//	}
//	return buf.Bytes(), nil
//}
