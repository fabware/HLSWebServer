#ifndef __TS_MUX_H__
#define __TS_MUX_H__

void print(int a);
int rawH264Data2Ts(void* data, unsigned int size, char** outdata, unsigned int *outLen);
void callback(void*f);
#endif