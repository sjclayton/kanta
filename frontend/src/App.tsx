import {useEffect, useState} from 'react';
import {PlaceholderGrid} from "../wailsjs/go/ui/App";
import './App.css';

type Grid = Awaited<ReturnType<typeof PlaceholderGrid>>;

function App() {
    const [grid, setGrid] = useState<Grid | null>(null);

    useEffect(() => {
        PlaceholderGrid().then(setGrid);
    }, []);

    if (!grid) {
        return <div id="App"><div className="placeholder">loading</div></div>;
    }

    const byRow: Record<number, typeof grid.keys> = {};
    for (const k of grid.keys) {
        (byRow[k.row] ??= []).push(k);
    }

    return (
        <div id="App">
            <h1 className="title">kanta</h1>
            <div
                className="grid"
                style={{
                    gridTemplateColumns: `repeat(${grid.width}, 1fr)`,
                    gridTemplateRows: `repeat(${grid.height}, 1fr)`,
                }}
            >
                {Object.keys(byRow).sort().map((r) =>
                    byRow[Number(r)].map((k) => (
                        <div key={`${k.row}-${k.col}`} className="key">{k.label}</div>
                    ))
                )}
            </div>
        </div>
    );
}

export default App;
