package decoder

import "io"

func ReadBytes(r io.Reader, n int) ([]byte, error) {
	result := make([]byte, 0)
	needToRead := n
	readed := 0
	for readed < needToRead {
		buff := make([]byte, needToRead)
		readed, err := r.Read(buff)
		if err != nil {
			return nil, err
		}

		result = append(result, buff[:readed]...)

		needToRead -= readed
	}

	return result, nil
}
