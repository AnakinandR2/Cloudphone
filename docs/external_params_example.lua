--[[
{
    "string_param": {
        "desc": "String parameter example",
        "type": "string",
        "required": true
    },
    "int_param": {
        "desc": "Integer parameter example",
        "type": "int",
        "required": true
    },
    "bool_param": {
        "desc": "Boolean parameter example",
        "type": "bool",
        "required": true
    },
    "array_param": {
        "desc": "Array parameter example. Inject as a Lua table, for example {'a', 'b', 'c'}",
        "type": "array",
        "required": true
    }
}
]]

local string_param = '${string_param}';
local int_param = ${int_param};
local bool_param = ${bool_param};
local array_param = ${array_param};

local function output(message)
    if type(log) == "function" then
        log(message);
    else
        print(message);
    end
end

local function serialize(value)
    local value_type = type(value);

    if value_type == "string" then
        return '"' .. value .. '"';
    end

    if value_type ~= "table" then
        return tostring(value);
    end

    local parts = {};
    for key, item in pairs(value) do
        local key_prefix = "";
        if type(key) ~= "number" then
            key_prefix = tostring(key) .. "=";
        end
        table.insert(parts, key_prefix .. serialize(item));
    end

    return "{" .. table.concat(parts, ", ") .. "}";
end

local params = {
    string_param = string_param,
    int_param = int_param,
    bool_param = bool_param,
    array_param = array_param
};

output("External parameter examples:");
output("string_param=" .. serialize(string_param));
output("int_param=" .. serialize(int_param));
output("bool_param=" .. serialize(bool_param));
output("array_param=" .. serialize(array_param));

if type(array_param) == "table" then
    output("array_param items:");
    for index, item in ipairs(array_param) do
        output("array_param[" .. tostring(index) .. "]=" .. serialize(item));
    end
else
    output("array_param is not a table: " .. serialize(array_param));
end

output("all_params=" .. serialize(params));