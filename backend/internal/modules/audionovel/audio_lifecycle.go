package audionovel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func removeAudioFile(directory, publicPath string) error {
	if publicPath == "" {
		return nil
	}
	if !audioNovelAudioPattern.MatchString(publicPath) {
		return fmt.Errorf("refusing to remove invalid audio path %q", publicPath)
	}
	// 只取已通过白名单校验的文件名，绝不让公开路径参与目录拼接。
	err := os.Remove(filepath.Join(directory, filepath.Base(publicPath)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
