# escape=`
FROM scratch AS base
COPY ["plain.txt", "name with space.txt", "/dest/"]
ADD sub/input.txt `
    ./app.txt /dest/
COPY <<-PAYLOAD /inline
	COPY fake-from-body /dest
	PAYLOAD
FROM base AS final
COPY --from=0 /dest /app
COPY --from=BASE /dest /other
RUN --mount=type=bind,from=base,source=/dest,target=/work echo ready
