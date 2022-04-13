import React, { useMemo } from 'react';
import { PanelProps } from '@grafana/data';
import { TraceView } from 'app/features/explore/TraceView/TraceView';
import { css } from '@emotion/css';
import { transformDataFrames } from 'app/features/explore/TraceView/utils/transform';
import { getDataSourceSrv } from '@grafana/runtime';
import { useAsync } from 'react-use';

const styles = {
  wrapper: css`
    height: 100%;
    overflow: scroll;
  `,
};

export const TracesPanel: React.FunctionComponent<PanelProps> = ({ data }) => {
  const traceProp = useMemo(() => transformDataFrames(data.series[0]), [data.series]);
  const dataSource = useAsync(async () => {
    return await getDataSourceSrv().get(data.request?.targets[0].datasource?.uid);
  });

  if (!data || !data.series.length || !traceProp) {
    return (
      <div className="panel-empty">
        <p>No data found in response</p>
      </div>
    );
  }

  return (
    <div className={styles.wrapper}>
      <TraceView dataFrames={data.series} queryResponse={data} traceProp={traceProp} datasource={dataSource.value} />
    </div>
  );
};
