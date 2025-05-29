const uint32_t sunny[4] = {
#ifdef jomla
    0xe0248222,
    0x881c703,
    0x104901f0,
    0x11
#else
    0x4442481f,
    0x71c1101,
    0xf0248444,
    66
#endif
};

const uint32_t cloudy[4] = {
#ifdef jomla
    0x30060000,
    0x20210107,
    0x3c0618,
    0x0
#else
    0xc033,
    0x84044043,
    0xc0f0000,
    66
#endif
};

const uint32_t rainy[4] = {
#ifdef jomla
    0x400c0000,
    0x400fc02,
    0x180080,
    0x0
#else
    0x6009,
    0x1f80200,
    0x20060000,
    66
#endif
};

const uint32_t snowy[4] = {
#ifdef jomla
    0xc8952124,
    0x2720fe09,
    0x490952,
    0x0
#else
    0x2489524e,
    0x43f84e49,
    0x52248000,
    66
#endif
};

const uint32_t foggy[4] = {
#ifdef jomla
    0x3f8000,
    0x7f00,
    0xe00007f0,
    0x7
#else
    0x3f800,
    0x7f00001,
    0xfc0003f0,
    66
#endif
};

