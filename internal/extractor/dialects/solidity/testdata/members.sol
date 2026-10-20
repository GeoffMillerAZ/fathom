// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IPool} from "./calls.sol";

contract Ownable {
    address internal owner;
    bool private ready;

    modifier once() {
        require(!ready);
        _;
        ready = true;
    }

    constructor(address first) once {
        owner = first;
    }
}

contract Treasury is Ownable {
    IPool private immutable pool;

    constructor(address p, address first) Ownable(first) {
        pool = IPool(p);
        setup(p);
    }

    receive() external payable {
        log(msg.value);
    }

    fallback() external payable {
        payable(owner).transfer(address(this).balance);
    }

    function setup(address p) private {
        IPool(p).swap(1);
    }

    function log(uint256 v) internal pure {}
}
