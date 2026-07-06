package extractor

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// EnsureAsset checks if targetDir exists. Returns skipped=true if already present.
func EnsureAsset(zipPath string, targetDir string) (bool, error) {
	if info, err := os.Stat(targetDir); err == nil && info.IsDir() {
		// Suppress verbose skip logs during silent startup checks unless explicitly debugging
		return true, nil
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return false, fmt.Errorf("failed to open zip file %s: %w", zipPath, err)
	}
	defer r.Close()

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create target dir: %w", err)
	}

	for _, f := range r.File {
		fpath := filepath.Join(targetDir, f.Name)

		if !filepath.HasPrefix(fpath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			return false, fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return false, err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return false, err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return false, err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return false, err
		}
	}

	return false, nil
}

// EnsureGoAsset checks if go/bin/go.exe exists inside workspaceDir. If not, unpacks zipPath directly into workspaceDir.
func EnsureGoAsset(zipPath string, workspaceDir string) (bool, error) {
	goExe := filepath.Join(workspaceDir, "go", "bin", "go.exe")
	if info, err := os.Stat(goExe); err == nil && !info.IsDir() {
		return true, nil
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return false, fmt.Errorf("failed to open zip file %s: %w", zipPath, err)
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(workspaceDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return false, err
		}
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return false, err
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return false, err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return false, err
		}
	}

	return false, nil
}

// EnsureGoRuntimeAsset checks if targetDir/bin/go.exe exists. If not, unpacks zipPath and strips the "go/" prefix from files.
func EnsureGoRuntimeAsset(zipPath string, targetDir string) (bool, error) {
	goExe := filepath.Join(targetDir, "bin", "go.exe")
	if info, err := os.Stat(goExe); err == nil && !info.IsDir() {
		return true, nil
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return false, fmt.Errorf("failed to open zip file %s: %w", zipPath, err)
	}
	defer r.Close()

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create target dir: %w", err)
	}

	for _, f := range r.File {
		// Go release zips always have a top-level "go" directory. Strip it.
		// f.Name might be "go/" or "go/bin/go.exe".
		if f.Name == "go/" {
			continue
		}
		
		cleanName := f.Name
		if len(cleanName) > 3 && cleanName[:3] == "go/" {
			cleanName = cleanName[3:]
		} else {
			// If for some reason it doesn't start with go/, just leave it
			continue
		}

		fpath := filepath.Join(targetDir, cleanName)
		
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return false, err
		}
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return false, err
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return false, err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return false, err
		}
	}

	return false, nil
}
