// +build go1.5

#define NOSPLIT 4

TEXT ·Wait(SB),NOSPLIT,$0-0
	B sync·runtime_Semacquire(SB)

TEXT ·Wake(SB),NOSPLIT,$0-0
	B runtime·semrelease(SB)
