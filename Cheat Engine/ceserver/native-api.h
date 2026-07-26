#ifndef NativeAPI_H_
#define NativeAPI_H_

#define MAX_HIT_COUNT  5000000
#define MAX_AOB_PATTERN_SIZE (1024 * 1024)
DWORD AOBScan(HANDLE hProcess, const char* pattern, const char* mask, int patternLength, uint64_t start, uint64_t end, int inc, int protection, uint64_t* match_addr);

#endif
