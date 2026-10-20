// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IPool {
    function swap(uint256 amount) external returns (uint256);
}

library Safe {
    function inc(uint256 x) internal pure returns (uint256) {
        return x + 1;
    }
}

contract Parent {
    event Moved(uint256 amount);
    error Empty();

    modifier nonZero(uint256 v) {
        if (v == 0) revert Empty();
        _;
    }

    function hook(uint256 v) internal virtual returns (uint256) {
        return v;
    }
}

contract Router is Parent {
    using Safe for uint256;

    IPool public pool;

    function route(uint256 amount) external nonZero(amount) returns (uint256) {
        uint256 bumped = amount.inc();
        uint256 out = pool.swap(bumped);
        emit Moved(out);
        return helper(out);
    }

    function helper(uint256 v) internal returns (uint256) {
        require(v > 0, "zero");
        uint256 r = super.hook(v);
        return keccak256(abi.encode(r)) == bytes32(0) ? 0 : Safe.inc(r);
    }

    function guarded(uint256 v) external pure {
        if (v == 0) revert Empty();
        uint256 w = uint256(v);
        w;
    }

    function checked(uint256 v, uint256[] storage list) internal {
        require(v > 0, Empty());
        list.push(v);
    }
}
