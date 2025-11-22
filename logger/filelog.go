package logger

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "sync/atomic"
    "time"
)

// Default file permissions for log files.
const defaultLogPerms = os.FileMode(0640)

type writerAndCloser interface {
    Write(b []byte) (int, error)
    Close() error
    Name() string
}

type FileLogger struct {
    currentSize       int64
    isRotationAllowed int32
    sync.Mutex
    logger                *Logger
    file                  writerAndCloser
    rotationLimit         int64
    originalRotationLimit int64
    processIDPrefix       string
    includeTimestamp      bool
    isClosed              bool
    maxBackupFiles        int
}

func newFileLogger(filename, processIDPrefix string, includeTimestamp bool) (*FileLogger, error) {
    fileflags := os.O_WRONLY | os.O_APPEND | os.O_CREATE
    file, err := os.OpenFile(filename, fileflags, defaultLogPerms)
    if err != nil {
        return nil, fmt.Errorf("unable to open log file %q: %w", filename, err)
    }

    stats, err := file.Stat()
    if err != nil {
        _ = file.Close()
        return nil, fmt.Errorf("unable to get file stats for %q: %w", filename, err)
    }

    fl := &FileLogger{
        isRotationAllowed: 0,
        file:              file,
        currentSize:       stats.Size(),
        processIDPrefix:   processIDPrefix,
        includeTimestamp:  includeTimestamp,
    }
    return fl, nil
}

func (fl *FileLogger) setLimit(limit int64) {
    fl.Lock()
    defer fl.Unlock()

    fl.originalRotationLimit, fl.rotationLimit = limit, limit
    atomic.StoreInt32(&fl.isRotationAllowed, 1)

    rotateNow := fl.currentSize > fl.rotationLimit
    if rotateNow && fl.logger != nil {
        fl.logger.Noticef("Rotating logfile...")
    }
}

func (fl *FileLogger) setMaxNumFiles(max int) {
    fl.Lock()
    defer fl.Unlock()
    fl.maxBackupFiles = max
}

func (fl *FileLogger) logDirect(label, format string, v ...any) int {
    var logBuffer = [256]byte{}
    logEntry := logBuffer[:0]

    if fl.processIDPrefix != "" {
        logEntry = append(logEntry, fl.processIDPrefix...)
    }

    if fl.includeTimestamp {
        now := time.Now()
        year, month, day := now.Date()
        hour, min, sec := now.Clock()
        microsec := now.Nanosecond() / 1000
        logEntry = append(logEntry,
            fmt.Sprintf("%04d/%02d/%02d %02d:%02d:%02d.%06d ",
                year, month, day, hour, min, sec, microsec)...,
        )
    }

    logEntry = append(logEntry, label...)
    logEntry = append(logEntry, fmt.Sprintf(format, v...)...)
    logEntry = append(logEntry, '\n')

    if fl.file == nil {
        // 极端情况下 file 为空时直接丢弃，避免 panic
        if fl.logger != nil {
            fl.logger.Warnf("FileLogger: logDirect called with nil file")
        }
        return len(logEntry)
    }

    _, err := fl.file.Write(logEntry)
    if err != nil && fl.logger != nil {
        fl.logger.Noticef("Error writing to log file: %v", err)
    }
    return len(logEntry)
}

func (fl *FileLogger) logPurge(fname string) {
    var backups []string
    logDir := filepath.Dir(fname)
    logBase := filepath.Base(fname)

    entries, err := os.ReadDir(logDir)
    if err != nil {
        if fl.logger != nil {
            fl.logDirect(fl.logger.errorLabel,
                "Unable to read directory %q for log purge (%v), will attempt next rotation",
                logDir, err,
            )
        }
        return
    }

    for _, entry := range entries {
        if entry.IsDir() || entry.Name() == logBase || !strings.HasPrefix(entry.Name(), logBase) {
            continue
        }
        if stamp, found := strings.CutPrefix(entry.Name(), logBase+"."); found {
            // stamp 形如 2006.01.02.15.04.05.999999999
            _, err := time.Parse("2006:01:02:15:04:05.999999999", strings.Replace(stamp, ".", ":", 5))
            if err == nil {
                backups = append(backups, entry.Name())
            }
        }
    }

    currBackups := len(backups)
    maxBackups := fl.maxBackupFiles - 1
    if currBackups > maxBackups {
        // backups 已按文件名排序（时间 + 名称），从最旧开始删
        for i := 0; i < currBackups-maxBackups; i++ {
            fullPath := filepath.Join(logDir, backups[i])
            if err := os.Remove(fullPath); err != nil {
                fl.logDirect(fl.logger.errorLabel,
                    "Unable to remove backup log file %q (%v), will attempt next rotation",
                    fullPath, err,
                )
                return
            }
            fl.logDirect(fl.logger.infoLabel, "Purged log file %q", fullPath)
        }
    }
}

func (fl *FileLogger) Write(b []byte) (int, error) {
    // 还没有开启 rotation 时，只做简单写入与计数
    if atomic.LoadInt32(&fl.isRotationAllowed) == 0 {
        n, err := fl.file.Write(b)
        if err != nil {
            return n, fmt.Errorf("error writing to log file: %w", err)
        }
        atomic.AddInt64(&fl.currentSize, int64(n))
        return n, nil
    }

    fl.Lock()
    defer fl.Unlock()

    // 原始写入
    n, err := fl.file.Write(b)
    if err != nil {
        return n, fmt.Errorf("error writing to log file during rotation: %w", err)
    }

    fl.currentSize += int64(n)

    // 检查是否需要轮转
    if fl.currentSize <= fl.rotationLimit {
        return n, nil
    }

    // 下面开始执行轮转流程
    if err := fl.file.Close(); err != nil {
        fl.rotationLimit *= 2
        if fl.logger != nil {
            fl.logDirect(fl.logger.errorLabel,
                "Unable to close logfile for rotation (%v), will attempt next rotation at size %v",
                err, fl.rotationLimit,
            )
        }
        return n, err
    }

    fname := fl.file.Name()
    now := time.Now()
    bak := fmt.Sprintf("%s.%04d.%02d.%02d.%02d.%02d.%02d.%09d",
        fname,
        now.Year(), now.Month(), now.Day(),
        now.Hour(), now.Minute(), now.Second(), now.Nanosecond(),
    )

    if err := os.Rename(fname, bak); err != nil {
        return n, fmt.Errorf("error renaming log file during rotation: %w", err)
    }

    fileflags := os.O_WRONLY | os.O_APPEND | os.O_CREATE
    file, err := os.OpenFile(fname, fileflags, defaultLogPerms)
    if err != nil {
        return n, fmt.Errorf("unable to re-open the logfile %q after rotation: %w", fname, err)
    }

    fl.file = file

    // 记录一次轮转成功的日志，这条日志的长度只用于 currentSize，不影响对外返回值
    if fl.logger != nil {
        rotatedLen := fl.logDirect(fl.logger.infoLabel, "Rotated log, backup saved as %q", bak)
        fl.currentSize = int64(rotatedLen)
    } else {
        fl.currentSize = 0
    }

    fl.rotationLimit = fl.originalRotationLimit

    if fl.maxBackupFiles > 0 {
        fl.logPurge(fname)
    }

    // 返回原始写入 b 的字节数和原始 err（此处为 nil）
    return n, nil
}

func (fl *FileLogger) close() error {
    fl.Lock()
    defer fl.Unlock()

    if fl.isClosed {
        return nil
    }

    fl.isClosed = true
    if err := fl.file.Close(); err != nil {
        return fmt.Errorf("error closing log file: %w", err)
    }
    return nil
}