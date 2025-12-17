package atserial

import (
	"fmt"
	"time"
	"syscall"
	"golang.org/x/sys/unix"
)

type PosixSerialPort struct {
	fd      int
	timeout time.Duration
}

func getBaudRateConstant(baudrate int) (uint32, error) {
	
	switch baudrate {
	case 9600:
		return unix.B9600, nil
	case 115200:
		return unix.B115200, nil
	case 230400:
		return unix.B230400, nil
	default:
		return 0, fmt.Errorf("unsupported baudrate: %d", baudrate)
	}
}

func OpenPosixSerial(portname string, baudrate int) (*PosixSerialPort, error) {

	fd, err := syscall.Open(portname, syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial port %s: %v", portname, err)
	}
	
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to get file flags: %v", err)
	}
	flags &^= unix.O_NONBLOCK
	if _, err := unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to set blocking mode: %v", err)
	}

	var termios unix.Termios
	if err := unix.IoctlSetTermios(fd, unix.TCGETS, &termios); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to get termios: %v", err)
	}

	speed, err := getBaudRateConstant(baudrate)
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}
	
	termios.Cflag &^= unix.CBAUD
	termios.Cflag |= speed

	
	termios.Cflag &^= unix.CSIZE
	termios.Cflag |= unix.CS8          
	termios.Cflag &^= unix.PARENB      
	termios.Cflag &^= unix.CSTOPB     

	termios.Cflag &^= unix.CRTSCTS

	termios.Cflag |= unix.CREAD | unix.CLOCAL

	termios.Lflag &^= unix.ICANON | unix.ECHO | unix.ECHOE | unix.ECHONL | unix.ISIG | unix.IEXTEN

	termios.Iflag &^= unix.IXON | unix.IXOFF | unix.IXANY
	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL

	termios.Oflag &^= unix.OPOST

	termios.Cc[unix.VMIN] = 0
	termios.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &termios); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to set termios: %v", err)
	}

	return &PosixSerialPort{
		fd:      fd,
		timeout: 100 * time.Millisecond, 
	}, nil
}

func (p *PosixSerialPort) Read(buf []byte) (int, error) {

	if p.timeout > 0 {

		fdSet := &unix.FdSet{}
		fdSet.Set(p.fd)

		timeout := &unix.Timeval{
			Sec:  int64(p.timeout / time.Second),
			Usec: int64(p.timeout % time.Second / time.Microsecond),
		}

		n, err := unix.Select(p.fd+1, fdSet, nil, nil, timeout)
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, fmt.Errorf("timeout")
		}
	}

	return unix.Read(p.fd, buf)
}

func (p *PosixSerialPort) Write(data []byte) (int, error) {
	return unix.Write(p.fd, data)
}

func (p *PosixSerialPort) Close() error {
	return syscall.Close(p.fd)
}

func (p *PosixSerialPort) SetReadTimeout(timeout time.Duration) {
	p.timeout = timeout
}
