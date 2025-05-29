#ifndef RPCLITE_DISPATCHER_H
#define RPCLITE_DISPATCHER_H

#include <map>
#include "wrapper.h"
#include "error.h"

struct DispatchEntry {
    MsgPack::str_t name;
    IFunctionWrapper* fn;
};

template<size_t N>
class RpcFunctionDispatcher {
public:
    template<typename F>
    bool bind(MsgPack::str_t name, F&& f) {
        if (_count >= N) return false;
        static auto wrapper = wrap(std::forward<F>(f));
        _entries[_count++] = {name, &wrapper};
        return true;
    }

    bool call(MsgPack::str_t name, MsgPack::Unpacker& unpacker, MsgPack::Packer& packer) {
        for (size_t i = 0; i < _count; ++i) {
            if (_entries[i].name == name) {
                return (*_entries[i].fn)(unpacker, packer);
            }
        }

        // handle not found
        MsgPack::object::nil_t nil;
        packer.serialize(RpcError(FUNCTION_NOT_FOUND_ERR, name), nil);
        return false;
    }

private:
    DispatchEntry _entries[N];
    size_t _count = 0;
};

#endif