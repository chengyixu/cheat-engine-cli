#include <stdlib.h>
#include <string.h>

#include "api.h"
#include "porthelp.h"
#include "ceserver.h"
#include "threads.h"
#include "symbols.h"
#include "context.h"
#include "native-api.h"

DWORD AOBScan(HANDLE hProcess, const char* pattern, const char* mask, int patternLength, uint64_t start, uint64_t end, int inc, int protection, uint64_t* match_addr) {
  const uint64_t chunkSize=1024*1024;
  int resultCount=0;
  uint64_t cursor=start;

  if ((pattern==NULL) || (mask==NULL) || (match_addr==NULL) || (patternLength<=0) || (patternLength>MAX_AOB_PATTERN_SIZE) || (inc<=0) || (end<=start))
    return 0;

  size_t bufferSize=(size_t)chunkSize+(size_t)patternLength-1;
  unsigned char *buffer=(unsigned char *)malloc(bufferSize);
  if (buffer==NULL)
    return 0;

  while (cursor<end)
  {
    RegionInfo rinfo={0};
    if (!VirtualQueryEx(hProcess, (void *)(uintptr_t)cursor, &rinfo, NULL) || (rinfo.size==0))
    {
      uint64_t next=cursor+4096;
      if (next<=cursor)
        break;
      cursor=next;
      continue;
    }

    uint64_t regionStart=rinfo.baseaddress>cursor ? rinfo.baseaddress : cursor;
    uint64_t regionEnd;
    if (UINT64_MAX-rinfo.baseaddress<rinfo.size)
      regionEnd=UINT64_MAX;
    else
      regionEnd=rinfo.baseaddress+rinfo.size;
    if (regionEnd>end)
      regionEnd=end;

    if (((rinfo.protection & protection)!=0) && (regionEnd>regionStart) && (regionEnd-regionStart>=(uint64_t)patternLength))
    {
      uint64_t address=regionStart;
      while (address<regionEnd)
      {
        uint64_t remaining=regionEnd-address;
        uint64_t requested=chunkSize+(uint64_t)patternLength-1;
        if (requested>remaining)
          requested=remaining;

        int bytesRead=ReadProcessMemory(hProcess, (void *)(uintptr_t)address, buffer, (int)requested);
        if (bytesRead>=patternLength)
        {
          int candidateCount=bytesRead-patternLength+1;
          if ((uint64_t)candidateCount>chunkSize)
            candidateCount=(int)chunkSize;

          for (int offset=0; offset<candidateCount; offset++)
          {
            uint64_t candidate=address+(uint64_t)offset;
            if (((candidate-start)%(uint64_t)inc)!=0)
              continue;

            int matched=1;
            for (int patternIndex=0; patternIndex<patternLength; patternIndex++)
            {
              if ((mask[patternIndex]!='?') && ((unsigned char)pattern[patternIndex]!=buffer[offset+patternIndex]))
              {
                matched=0;
                break;
              }
            }
            if (matched)
            {
              match_addr[resultCount++]=candidate;
              if (resultCount>=MAX_HIT_COUNT)
              {
                free(buffer);
                return resultCount;
              }
            }
          }
        }

        uint64_t next=address+chunkSize;
        if (next<=address)
          break;
        address=next;
      }
    }

    if (regionEnd<=cursor)
    {
      uint64_t next=cursor+4096;
      if (next<=cursor)
        break;
      cursor=next;
    }
    else
      cursor=regionEnd;
  }

  free(buffer);
  return resultCount;
}
