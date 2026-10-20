// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

type Price is uint128;

uint256 constant MAX_SUPPLY = 1000;

error Unauthorized(address caller);

event Created(address indexed owner, uint256 id);

struct Config {
    uint256 fee;
    address treasury;
}

enum Status {
    Active,
    Paused
}

function clamp(uint256 x, uint256 hi) pure returns (uint256) {
    return x > hi ? hi : x;
}

interface IToken {
    event Transfer(address indexed from, address indexed to, uint256 value);
    error Insufficient(uint256 have, uint256 want);

    function balanceOf(address who) external view returns (uint256);
}

library MathLib {
    struct Pair {
        uint256 a;
        uint256 b;
    }

    function add(uint256 a, uint256 b) internal pure returns (uint256) {
        return a + b;
    }
}

abstract contract Base {
    address public owner;
    uint256 internal constant LIMIT = 10;

    modifier onlyOwner() {
        if (msg.sender != owner) revert Unauthorized(msg.sender);
        _;
    }

    constructor(address first) {
        owner = first;
    }

    function setOwner(address next) public virtual onlyOwner {
        owner = next;
    }
}

contract Vault is Base, IToken {
    enum Mode { Open, Closed }

    mapping(address => uint256) private balances;
    Mode public mode;

    event Deposited(address indexed from, uint256 amount);
    error Closed();

    function deposit(
        uint256 amount,
        address from
    ) external payable returns (bool ok) {
        balances[from] += amount;
        emit Deposited(from, amount);
        return true;
    }

    function balanceOf(address who) external view override returns (uint256) {
        return balances[who];
    }

    receive() external payable {}
}
