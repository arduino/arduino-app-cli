#ifndef RPCLITE_WRAPPER_H
#define RPCLITE_WRAPPER_H

#include "error.h"

#ifdef HANDLE_RPC_ERRORS
#include <stdexcept>
#endif

//TODO maybe use arx::function_traits

// C++11-compatible function_traits
// Primary template: fallback
template<typename T>
struct function_traits;

// Function pointer
template<typename R, typename... Args>
struct function_traits<R(*)(Args...)> {
    using signature = R(Args...);
};

// std::function
template<typename R, typename... Args>
struct function_traits<std::function<R(Args...)>> {
    using signature = R(Args...);
};

// Member function pointer (including lambdas)
template<typename C, typename R, typename... Args>
struct function_traits<R(C::*)(Args...) const> {
    using signature = R(Args...);
};

// Deduction helper for lambdas
template<typename T>
struct function_traits {
    using signature = typename function_traits<decltype(&T::operator())>::signature;
};


// Helper to invoke a function with a tuple of arguments
template<typename F, typename Tuple, std::size_t... I>
auto invoke_with_tuple(F&& f, Tuple&& t, arx::stdx::index_sequence<I...>)
    -> decltype(f(std::get<I>(std::forward<Tuple>(t))...)) {
    return f(std::get<I>(std::forward<Tuple>(t))...);
};

template<typename F>
class RpcFunctionWrapper;

class IFunctionWrapper {
    public:
        virtual ~IFunctionWrapper() {}
        virtual bool operator()(MsgPack::Unpacker& unpacker, MsgPack::Packer& packer) = 0;
    };

template<typename R, typename... Args>
class RpcFunctionWrapper<R(Args...)>: public IFunctionWrapper {
public:
    RpcFunctionWrapper(std::function<R(Args...)> func) : _func(func) {}

    R operator()(Args... args) {
        return _func(args...);
    }

    bool operator()(MsgPack::Unpacker& unpacker, MsgPack::Packer& packer) override {

        MsgPack::object::nil_t nil;

#ifdef HANDLE_RPC_ERRORS
    try {
#endif

        // First check the parameters size
        if (!unpacker.isArray()){
            RpcError error(MALFORMED_CALL_ERR, "Unserializable parameters array");
            packer.serialize(error, nil);
            return false;
        }

        MsgPack::arr_size_t param_size;

        unpacker.deserialize(param_size);
        if (param_size.size() < sizeof...(Args)){
            RpcError error(MALFORMED_CALL_ERR, "Missing call parameters (WARNING: Default param resolution is not implemented)");
            packer.serialize(error, nil);
            return false;
        }

        if (param_size.size() > sizeof...(Args)){
            RpcError error(MALFORMED_CALL_ERR, "Too many parameters");
            packer.serialize(error, nil);
            return false;
        }

        return handle_call(unpacker, packer);

#ifdef HANDLE_RPC_ERRORS
    } catch (const std::exception& e) {
        // Should be specialized according to the exception type
        RpcError error(GENERIC_ERR, "RPC error");
        packer.serialize(error, nil);
        return false;
    }
#endif

    }

private:
    std::function<R(Args...)> _func;

    template<typename Dummy = R>
    typename std::enable_if<std::is_void<Dummy>::value, bool>::type
    handle_call(MsgPack::Unpacker& unpacker, MsgPack::Packer& packer) {
        //unpacker not ready if deserialization fails at this point
        std::tuple<Args...> args;
        if (!deserialize_all<Args...>(unpacker, args)) return false;
        MsgPack::object::nil_t nil;
        invoke_with_tuple(_func, args, arx::stdx::make_index_sequence<sizeof...(Args)>{});
        packer.serialize(nil, nil);
        return true;
    }

    template<typename Dummy = R>
    typename std::enable_if<!std::is_void<Dummy>::value, bool>::type
    handle_call(MsgPack::Unpacker& unpacker, MsgPack::Packer& packer) {
        //unpacker not ready if deserialization fails at this point
        std::tuple<Args...> args;
        if (!deserialize_all<Args...>(unpacker, args)) return false;
        MsgPack::object::nil_t nil;
        R out = invoke_with_tuple(_func, args, arx::stdx::make_index_sequence<sizeof...(Args)>{});
        packer.serialize(nil, out);
        return true;
    }

    template<size_t I = 0, typename... Ts>
    inline typename std::enable_if<I == sizeof...(Ts), bool>::type
    deserialize_tuple(MsgPack::Unpacker& unpacker, std::tuple<Ts...>& out) {
        (void)unpacker;    // silence unused parameter warning
        (void)out;
        return true;        // Base case
    }

    template<size_t I = 0, typename... Ts>
    inline typename std::enable_if<I < sizeof...(Ts), bool>::type
    deserialize_tuple(MsgPack::Unpacker& unpacker, std::tuple<Ts...>& out) {
        if (!deserialize_single(unpacker, std::get<I>(out))) {
            return false;
        }
        return deserialize_tuple<I+1>(unpacker, out); // Recursive unpacking
    }

    template<typename... Ts>
    bool deserialize_all(MsgPack::Unpacker& unpacker, std::tuple<Ts...>& values) {
        return deserialize_tuple(unpacker, values);
    }

    template<typename T>
    static bool deserialize_single(MsgPack::Unpacker& unpacker, T& value) {
        if (!unpacker.unpackable(value)) return false;
        unpacker.deserialize(value);
        return true;
    }
};


template<typename F>
auto wrap(F&& f) -> RpcFunctionWrapper<typename function_traits<typename std::decay<F>::type>::signature> {
    using Signature = typename function_traits<typename std::decay<F>::type>::signature;
    return RpcFunctionWrapper<Signature>(std::forward<F>(f));
};

#endif