% SCRIPT  Run the demo.
clear; close all;
x = 1:10;
y = x .^ 2;
plot(x, y);

function plot_local(a)
    plot(a);
end
